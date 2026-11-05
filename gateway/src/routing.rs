use std::sync::Arc;

use anyhow::{bail, Context, Result};
use redis::AsyncCommands;
use tokio::sync::OnceCell;

#[derive(Clone)]
pub struct RoomRouter {
    inner: Arc<Inner>,
}

struct Inner {
    client: redis::Client,
    prefix: String,
    connection: OnceCell<redis::aio::MultiplexedConnection>,
}

impl RoomRouter {
    pub fn new(redis_url: &str, prefix: String) -> Result<Self> {
        Ok(Self {
            inner: Arc::new(Inner {
                client: redis::Client::open(redis_url).context("parse REDIS_URL")?,
                prefix,
                connection: OnceCell::new(),
            }),
        })
    }

    async fn connection(&self) -> Result<redis::aio::MultiplexedConnection> {
        Ok(self
            .inner
            .connection
            .get_or_try_init(|| async {
                self.inner
                    .client
                    .get_multiplexed_async_connection()
                    .await
                    .context("connect Redis")
            })
            .await?
            .clone())
    }

    fn prefix(&self, namespace: Option<&str>) -> String {
        match namespace.filter(|value| !value.is_empty()) {
            Some(namespace) => format!("{}:{namespace}", self.inner.prefix),
            None => self.inner.prefix.clone(),
        }
    }

    pub async fn route(&self, room_id: &str, namespace: Option<&str>) -> Result<String> {
        if room_id.is_empty() || room_id.len() > 128 {
            bail!("missing or invalid room ID");
        }
        let prefix = self.prefix(namespace);
        let mut connection = self.connection().await?;
        let nodes_key = format!("{prefix}:nodes");
        let mut candidates = connection.smembers::<_, Vec<String>>(&nodes_key).await?;
        if candidates.is_empty() {
            bail!("no livehouse nodes are available");
        }
        candidates.sort();
        let offset = stable_hash(room_id.as_bytes()) as usize % candidates.len();
        candidates.rotate_left(offset);
        let script = redis::Script::new(
            r#"
            local current = redis.call('GET', KEYS[1])
            if current then
                local address = redis.call('GET', ARGV[1] .. current)
                if address then
                    redis.call('EXPIRE', KEYS[1], ARGV[2])
                    return address
                end
            end
            for index = 3, #ARGV do
                local address = redis.call('GET', ARGV[1] .. ARGV[index])
                if address then
                    redis.call('SET', KEYS[1], ARGV[index], 'EX', ARGV[2])
                    return address
                end
            end
            return nil
            "#,
        );
        let mut invocation = script.key(format!("{prefix}:room:{room_id}:node"));
        invocation
            .arg(format!("{prefix}:node:"))
            .arg(86_400)
            .arg(candidates);
        let address = invocation
            .invoke_async::<Option<String>>(&mut connection)
            .await?
            .ok_or_else(|| anyhow::anyhow!("no healthy livehouse nodes are available"))?;
        Ok(address)
    }

    pub async fn invalidate(&self, room_id: &str, namespace: Option<&str>) -> Result<()> {
        let prefix = self.prefix(namespace);
        let mut connection = self.connection().await?;
        connection
            .del::<_, ()>(format!("{prefix}:room:{room_id}:node"))
            .await?;
        Ok(())
    }
}

fn stable_hash(bytes: &[u8]) -> u64 {
    bytes.iter().fold(0xcbf29ce484222325, |hash, byte| {
        (hash ^ u64::from(*byte)).wrapping_mul(0x100000001b3)
    })
}

pub fn room_id_from_query(query: Option<&str>) -> Option<String> {
    url::form_urlencoded::parse(query.unwrap_or_default().as_bytes()).find_map(|(key, value)| {
        matches!(key.as_ref(), "roomId" | "houseId").then(|| value.into_owned())
    })
}
