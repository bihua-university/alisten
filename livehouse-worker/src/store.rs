use std::sync::Arc;

use anyhow::{Context, Result};
use async_trait::async_trait;
use redis::{AsyncCommands, Script};
use serde_json;

use crate::model::PersistedRoom;

pub const LEASE_TTL_MS: u64 = 10_000;
pub const NODE_TTL_SECONDS: u64 = 15;

#[async_trait]
pub trait StateStore: Send + Sync {
    async fn create_room(&self, room: &PersistedRoom) -> Result<bool>;
    async fn get_room(&self, room_id: &str) -> Result<Option<PersistedRoom>>;
    async fn list_rooms(&self) -> Result<Vec<PersistedRoom>>;
    async fn acquire(&self, room_id: &str, node_id: &str) -> Result<Option<PersistedRoom>>;
    async fn commit(&self, room: &PersistedRoom, expected: u64, node_id: &str) -> Result<bool>;
    async fn renew(&self, room_id: &str, node_id: &str) -> Result<bool>;
    async fn release(&self, room_id: &str, node_id: &str) -> Result<()>;
    async fn delete_room(&self, room_id: &str, node_id: &str) -> Result<()>;
}

#[derive(Clone)]
pub struct RedisStore {
    connection: redis::aio::MultiplexedConnection,
    prefix: Arc<str>,
}

impl RedisStore {
    pub async fn connect(url: &str, prefix: String) -> Result<Self> {
        let client = redis::Client::open(url).context("parse REDIS_URL")?;
        let connection = client
            .get_multiplexed_async_connection()
            .await
            .context("connect Redis")?;
        Ok(Self {
            connection,
            prefix: Arc::from(prefix),
        })
    }

    fn state_key(&self, room_id: &str) -> String {
        format!("{}:state:{room_id}", self.prefix)
    }

    fn lease_key(&self, room_id: &str) -> String {
        format!("{}:lease:{room_id}", self.prefix)
    }

    fn node_key(&self, node_id: &str) -> String {
        format!("{}:node:{node_id}", self.prefix)
    }

    pub async fn heartbeat(&self, node_id: &str, address: &str) -> Result<()> {
        let mut connection = self.connection.clone();
        redis::pipe()
            .atomic()
            .set_ex(self.node_key(node_id), address, NODE_TTL_SECONDS)
            .sadd(format!("{}:nodes", self.prefix), node_id)
            .query_async::<()>(&mut connection)
            .await?;
        Ok(())
    }

    pub async fn assigned_node(&self, room_id: &str, node_id: &str) -> Result<(String, String)> {
        let script = Script::new(
            r#"
            local current = redis.call('GET', KEYS[1])
            if current then
                local address = redis.call('GET', ARGV[2] .. current)
                if address then
                    redis.call('EXPIRE', KEYS[1], 86400)
                    return {current, address}
                end
            end
            redis.call('SET', KEYS[1], ARGV[1], 'EX', 86400)
            local address = redis.call('GET', ARGV[2] .. ARGV[1])
            if not address then
                redis.call('DEL', KEYS[1])
                return nil
            end
            return {ARGV[1], address}
            "#,
        );
        let mut connection = self.connection.clone();
        let assigned = script
            .key(format!("{}:room:{room_id}:node", self.prefix))
            .arg(node_id)
            .arg(format!("{}:node:", self.prefix))
            .invoke_async::<Option<(String, String)>>(&mut connection)
            .await?
            .ok_or_else(|| anyhow::anyhow!("current livehouse node has no heartbeat"))?;
        Ok(assigned)
    }

    pub async fn unregister_node(&self, node_id: &str) -> Result<()> {
        let mut connection = self.connection.clone();
        redis::pipe()
            .atomic()
            .del(self.node_key(node_id))
            .srem(format!("{}:nodes", self.prefix), node_id)
            .query_async::<()>(&mut connection)
            .await?;
        Ok(())
    }

    async fn scan_keys(&self, pattern: String) -> Result<Vec<String>> {
        let mut connection = self.connection.clone();
        let mut cursor = 0_u64;
        let mut keys = Vec::new();
        loop {
            let (next, mut page): (u64, Vec<String>) = redis::cmd("SCAN")
                .arg(cursor)
                .arg("MATCH")
                .arg(&pattern)
                .arg("COUNT")
                .arg(200)
                .query_async(&mut connection)
                .await?;
            keys.append(&mut page);
            if next == 0 {
                return Ok(keys);
            }
            cursor = next;
        }
    }
}

#[async_trait]
impl StateStore for RedisStore {
    async fn create_room(&self, room: &PersistedRoom) -> Result<bool> {
        let mut connection = self.connection.clone();
        let raw = serde_json::to_string(room)?;
        let created = connection
            .set_nx::<_, _, bool>(self.state_key(&room.id), raw)
            .await?;
        Ok(created)
    }

    async fn get_room(&self, room_id: &str) -> Result<Option<PersistedRoom>> {
        let mut connection = self.connection.clone();
        let raw: Option<String> = connection.get(self.state_key(room_id)).await?;
        raw.map(|raw| serde_json::from_str(&raw).context("decode room state"))
            .transpose()
    }

    async fn list_rooms(&self) -> Result<Vec<PersistedRoom>> {
        let keys = self.scan_keys(format!("{}:state:*", self.prefix)).await?;
        if keys.is_empty() {
            return Ok(Vec::new());
        }
        let mut connection = self.connection.clone();
        let values: Vec<Option<String>> = connection.get(keys).await?;
        values
            .into_iter()
            .flatten()
            .map(|raw| serde_json::from_str(&raw).context("decode room state"))
            .collect()
    }

    async fn acquire(&self, room_id: &str, node_id: &str) -> Result<Option<PersistedRoom>> {
        let script = Script::new(
            r#"
            local owner = redis.call('GET', KEYS[1])
            if owner and owner ~= ARGV[1] and redis.call('PTTL', KEYS[1]) > 0 then
                return 0
            end
            redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
            return 1
            "#,
        );
        let mut connection = self.connection.clone();
        let acquired: u8 = script
            .key(self.lease_key(room_id))
            .arg(node_id)
            .arg(LEASE_TTL_MS)
            .invoke_async(&mut connection)
            .await?;
        if acquired == 0 {
            return Ok(None);
        }
        self.get_room(room_id).await
    }

    async fn commit(&self, room: &PersistedRoom, expected: u64, node_id: &str) -> Result<bool> {
        let script = Script::new(
            r#"
            if redis.call('GET', KEYS[1]) ~= ARGV[1] then
                return 0
            end
            local current = redis.call('GET', KEYS[2])
            if not current or cjson.decode(current)['revision'] ~= tonumber(ARGV[2]) then
                return 0
            end
            redis.call('SET', KEYS[2], ARGV[3])
            return 1
            "#,
        );
        let mut connection = self.connection.clone();
        let committed: u8 = script
            .key(self.lease_key(&room.id))
            .key(self.state_key(&room.id))
            .arg(node_id)
            .arg(expected)
            .arg(serde_json::to_string(room)?)
            .invoke_async(&mut connection)
            .await?;
        Ok(committed == 1)
    }

    async fn renew(&self, room_id: &str, node_id: &str) -> Result<bool> {
        let script = Script::new(
            "if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('PEXPIRE', KEYS[1], ARGV[2]) else return 0 end",
        );
        let mut connection = self.connection.clone();
        let renewed: u8 = script
            .key(self.lease_key(room_id))
            .arg(node_id)
            .arg(LEASE_TTL_MS)
            .invoke_async(&mut connection)
            .await?;
        Ok(renewed == 1)
    }

    async fn release(&self, room_id: &str, node_id: &str) -> Result<()> {
        let script = Script::new(
            "if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) else return 0 end",
        );
        let mut connection = self.connection.clone();
        let _: u8 = script
            .key(self.lease_key(room_id))
            .arg(node_id)
            .invoke_async(&mut connection)
            .await?;
        Ok(())
    }

    async fn delete_room(&self, room_id: &str, node_id: &str) -> Result<()> {
        let script = Script::new(
            r#"
            if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
            redis.call('DEL', KEYS[1], KEYS[2])
            redis.call('DEL', ARGV[2])
            return 1
            "#,
        );
        let mut connection = self.connection.clone();
        let _: u8 = script
            .key(self.lease_key(room_id))
            .key(self.state_key(room_id))
            .arg(node_id)
            .arg(format!("{}:room:{}:node", self.prefix, room_id))
            .invoke_async(&mut connection)
            .await?;
        Ok(())
    }
}

#[cfg(test)]
#[derive(Default)]
pub struct MemoryStore {
    rooms: std::sync::Mutex<std::collections::HashMap<String, PersistedRoom>>,
    leases: std::sync::Mutex<std::collections::HashMap<String, String>>,
}

#[cfg(test)]
impl MemoryStore {
    fn rooms(&self) -> std::sync::MutexGuard<'_, std::collections::HashMap<String, PersistedRoom>> {
        self.rooms.lock().unwrap()
    }
}

#[cfg(test)]
#[async_trait]
impl StateStore for MemoryStore {
    async fn create_room(&self, room: &PersistedRoom) -> Result<bool> {
        let mut rooms = self.rooms();
        if rooms.contains_key(&room.id) {
            return Ok(false);
        }
        rooms.insert(room.id.clone(), room.clone());
        Ok(true)
    }

    async fn get_room(&self, room_id: &str) -> Result<Option<PersistedRoom>> {
        Ok(self.rooms().get(room_id).cloned())
    }

    async fn list_rooms(&self) -> Result<Vec<PersistedRoom>> {
        Ok(self.rooms().values().cloned().collect())
    }

    async fn acquire(&self, room_id: &str, node_id: &str) -> Result<Option<PersistedRoom>> {
        let mut leases = self.leases.lock().unwrap();
        if leases
            .get(room_id)
            .is_some_and(|current| current != node_id)
        {
            return Ok(None);
        }
        leases.insert(room_id.to_owned(), node_id.to_owned());
        drop(leases);
        Ok(self.rooms().get(room_id).cloned())
    }

    async fn commit(&self, room: &PersistedRoom, expected: u64, node_id: &str) -> Result<bool> {
        if self
            .leases
            .lock()
            .unwrap()
            .get(&room.id)
            .map(String::as_str)
            != Some(node_id)
        {
            return Ok(false);
        }
        let mut rooms = self.rooms();
        let Some(current) = rooms.get(&room.id) else {
            return Ok(false);
        };
        if current.revision != expected {
            return Ok(false);
        }
        rooms.insert(room.id.clone(), room.clone());
        Ok(true)
    }

    async fn renew(&self, room_id: &str, node_id: &str) -> Result<bool> {
        Ok(self.leases.lock().unwrap().get(room_id).map(String::as_str) == Some(node_id))
    }

    async fn release(&self, room_id: &str, node_id: &str) -> Result<()> {
        let mut leases = self.leases.lock().unwrap();
        if leases.get(room_id).map(String::as_str) == Some(node_id) {
            leases.remove(room_id);
        }
        Ok(())
    }

    async fn delete_room(&self, room_id: &str, node_id: &str) -> Result<()> {
        self.release(room_id, node_id).await?;
        self.rooms().remove(room_id);
        Ok(())
    }
}
