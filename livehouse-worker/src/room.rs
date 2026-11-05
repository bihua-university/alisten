use std::{
    collections::HashMap,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc,
    },
    time::Duration,
};

use serde_json::{json, Value};
use tokio::sync::{broadcast, Mutex, RwLock};
use uuid::Uuid;

use crate::{
    model::{
        now_ms, ChatData, LikeResult, MembersData, PersistedRoom, PickResult, PlayMode,
        PlaybackAnchor, PlaybackData, PlaybackStatus, QueueData, RoomInfo, RoomSnapshot,
        SettingsData, SongRef, Track, User, VoteResult, CHAT_HISTORY_LIMIT,
    },
    protocol::{event, ErrorCode, LivehouseError},
    store::StateStore,
};

const BROADCAST_CAPACITY: usize = 128;
const MAX_LAGGED_ROOM_EVENTS: u32 = 3;
pub const EMPTY_ROOM_GRACE_MS: u64 = 5 * 60 * 1000;

pub struct Room {
    id: String,
    node_id: String,
    store: Arc<dyn StateStore>,
    state: RwLock<PersistedRoom>,
    members: RwLock<HashMap<u64, User>>,
    mutation: Mutex<()>,
    events: broadcast::Sender<Arc<str>>,
    closed: AtomicBool,
    last_active_ms: Mutex<u64>,
    max_members: usize,
}

impl Room {
    fn new(
        state: PersistedRoom,
        node_id: String,
        store: Arc<dyn StateStore>,
        max_members: usize,
    ) -> Self {
        let (events, _) = broadcast::channel(BROADCAST_CAPACITY);
        Self {
            id: state.id.clone(),
            node_id,
            store,
            state: RwLock::new(state),
            members: RwLock::new(HashMap::new()),
            mutation: Mutex::new(()),
            events,
            closed: AtomicBool::new(false),
            last_active_ms: Mutex::new(now_ms()),
            max_members,
        }
    }

    pub fn is_closed(&self) -> bool {
        self.closed.load(Ordering::Acquire)
    }

    pub async fn info(&self) -> RoomInfo {
        self.state.read().await.info()
    }

    pub async fn password_matches(&self, password: &str) -> bool {
        self.state.read().await.password == password
    }

    async fn members_data(&self) -> MembersData {
        let members = self.members.read().await;
        MembersData {
            users: members.values().cloned().collect(),
            online_count: members.len() as u64,
        }
    }

    pub async fn snapshot(&self) -> RoomSnapshot {
        let state = self.state.read().await;
        let members = self.members_data().await;
        RoomSnapshot {
            revision: state.revision,
            room: state.info(),
            queue: state.queue(),
            playback: state.playback_at(now_ms()),
            members,
            settings: SettingsData {
                play_mode: state.play_mode,
            },
            chat: state.chat.iter().cloned().collect(),
            server_time_ms: now_ms(),
        }
    }

    pub async fn queue(&self) -> QueueData {
        self.state.read().await.queue()
    }

    pub async fn playback(&self) -> PlaybackData {
        self.state.read().await.playback_at(now_ms())
    }

    pub async fn settings(&self) -> SettingsData {
        SettingsData {
            play_mode: self.state.read().await.play_mode,
        }
    }

    pub async fn members(&self) -> MembersData {
        self.members_data().await
    }

    async fn mutate<T, F>(&self, mutate: F) -> Result<T, LivehouseError>
    where
        F: FnOnce(&mut PersistedRoom, u64) -> Result<(T, Vec<Value>), LivehouseError>,
    {
        let _mutation = self.mutation.lock().await;
        if self.is_closed() {
            return Err(LivehouseError::new(
                ErrorCode::LeaseLost,
                "房间已迁移，请重新连接",
            ));
        }
        let (result, next, events) = {
            let mut state = self.state.write().await;
            let expected = state.revision;
            let revision = expected.saturating_add(1);
            let (result, events) = mutate(&mut state, revision)?;
            state.revision = revision;
            (result, state.clone(), events)
        };
        let committed = match self
            .store
            .commit(&next, next.revision.saturating_sub(1), &self.node_id)
            .await
        {
            Ok(committed) => committed,
            Err(error) => {
                self.lose_lease();
                return Err(LivehouseError::unavailable(format!(
                    "房间状态持久化失败: {error}"
                )));
            }
        };
        if !committed {
            self.lose_lease();
            return Err(LivehouseError::new(
                ErrorCode::LeaseLost,
                "房间租约已失效，请重新连接",
            ));
        }
        for event in events {
            self.broadcast(event);
        }
        Ok(result)
    }

    fn broadcast(&self, message: Value) {
        let raw = Arc::<str>::from(message.to_string());
        let _ = self.events.send(raw);
    }

    fn lose_lease(&self) {
        if !self.closed.swap(true, Ordering::AcqRel) {
            self.broadcast(event(
                "room.closed",
                json!({ "reason": "lease_lost", "message": "房间已迁移，请重新连接" }),
            ));
        }
    }

    pub async fn join(
        &self,
        member_id: u64,
        user: User,
    ) -> Result<(broadcast::Receiver<Arc<str>>, RoomSnapshot), LivehouseError> {
        let receiver = self.events.subscribe();
        {
            let members = self.members.read().await;
            if members.len() >= self.max_members && !members.contains_key(&member_id) {
                return Err(LivehouseError::new(
                    ErrorCode::RateLimited,
                    "房间成员已达到上限",
                ));
            }
        }
        self.members.write().await.insert(member_id, user);
        let population = self.members.read().await.len() as u64;
        let revision = self.state.read().await.revision.saturating_add(1);
        let members = self.members_data().await;
        let result = self
            .mutate(move |state, _| {
                state.population = population;
                if state.owner.is_none() {
                    state.owner = members.users.first().cloned();
                }
                Ok((
                    (),
                    vec![event(
                        "members.changed",
                        json!({ "revision": revision, "members": members }),
                    )],
                ))
            })
            .await;
        if let Err(error) = result {
            self.members.write().await.remove(&member_id);
            return Err(error);
        }
        *self.last_active_ms.lock().await = now_ms();
        Ok((receiver, self.snapshot().await))
    }

    pub async fn leave(&self, member_id: u64) {
        if self.members.write().await.remove(&member_id).is_none() || self.is_closed() {
            return;
        }
        let population = self.members.read().await.len() as u64;
        let revision = self.state.read().await.revision.saturating_add(1);
        let members = self.members_data().await;
        let _ = self
            .mutate(move |state, _| {
                state.population = population;
                Ok((
                    (),
                    vec![event(
                        "members.changed",
                        json!({ "revision": revision, "members": members }),
                    )],
                ))
            })
            .await;
        *self.last_active_ms.lock().await = now_ms();
    }

    pub async fn update_user(
        &self,
        member_id: u64,
        user: User,
    ) -> Result<MembersData, LivehouseError> {
        if !self.members.read().await.contains_key(&member_id) {
            return Err(LivehouseError::not_found("成员已离开房间"));
        }
        self.members.write().await.insert(member_id, user);
        let revision = self.state.read().await.revision.saturating_add(1);
        let members = self.members_data().await;
        self.mutate(move |state, _| {
            if state.owner.is_none() {
                state.owner = members.users.first().cloned();
            }
            Ok((
                members.clone(),
                vec![event(
                    "members.changed",
                    json!({ "revision": revision, "members": members }),
                )],
            ))
        })
        .await
    }

    pub async fn enqueue(&self, track: Track) -> Result<PickResult, LivehouseError> {
        let result = PickResult {
            id: track.playable.song.id.clone(),
            name: track.playable.song.name.clone(),
            artist: if track.playable.song.artist.is_empty() {
                "unknown".to_owned()
            } else {
                track.playable.song.artist.clone()
            },
            source: track.playable.song.source,
        };
        let now = now_ms();
        self.mutate(move |state, revision| {
            if state
                .current
                .iter()
                .chain(state.queue.iter())
                .any(|queued| queued.key() == track.key())
            {
                return Err(LivehouseError::conflict("重复点歌"));
            }
            let starts_playing = state.current.is_none();
            if !starts_playing && !state.ultimate && state.queue.len() >= 10 {
                return Err(LivehouseError::conflict(
                    "已超过最大点歌数量10首，请稍后再试",
                ));
            }
            if starts_playing {
                state.playback = PlaybackAnchor {
                    status: PlaybackStatus::Playing,
                    started_at_ms: now.saturating_add(200),
                    position_ms: 0,
                };
                state.current = Some(track);
            } else {
                state.queue.push_back(track);
            }
            let queue = event(
                "queue.changed",
                json!({ "revision": revision, "queue": state.queue() }),
            );
            let mut events = vec![queue];
            if starts_playing {
                events.push(event(
                    "playback.changed",
                    json!({ "revision": revision, "playback": state.playback_at(now) }),
                ));
            }
            Ok((result.clone(), events))
        })
        .await
    }

    pub async fn remove(&self, reference: SongRef) -> Result<Track, LivehouseError> {
        self.mutate(move |state, revision| {
            let index = state
                .queue
                .iter()
                .position(|track| {
                    track.playable.song.id == reference.id
                        && track.playable.song.source == reference.source
                })
                .ok_or_else(|| LivehouseError::not_found("未找到要删除的音乐"))?;
            let removed = state.queue.remove(index).expect("queue index checked");
            Ok((
                removed,
                vec![event(
                    "queue.changed",
                    json!({ "revision": revision, "queue": state.queue() }),
                )],
            ))
        })
        .await
    }

    pub async fn like(&self, reference: SongRef) -> Result<LikeResult, LivehouseError> {
        self.mutate(move |state, revision| {
            let track = state
                .queue
                .iter_mut()
                .find(|track| {
                    track.playable.song.id == reference.id
                        && track.playable.song.source == reference.source
                })
                .ok_or_else(|| LivehouseError::not_found("未找到对应音乐"))?;
            track.likes = track.likes.saturating_add(1);
            let result = LikeResult {
                id: reference.id,
                source: reference.source,
                name: track.playable.song.name.clone(),
                likes: track.likes,
            };
            state
                .queue
                .make_contiguous()
                .sort_by(|left, right| right.likes.cmp(&left.likes));
            Ok((
                result,
                vec![event(
                    "queue.changed",
                    json!({ "revision": revision, "queue": state.queue() }),
                )],
            ))
        })
        .await
    }

    pub async fn vote(&self, user: User) -> Result<VoteResult, LivehouseError> {
        let now = now_ms();
        self.mutate(move |state, revision| {
            let population = state.population.max(1);
            let required = ((population + 2) / 3).max(1);
            let current = state
                .current
                .as_mut()
                .ok_or_else(|| LivehouseError::not_found("当前没有播放音乐"))?;
            if !current.voters.insert(user.key()) {
                return Err(LivehouseError::conflict("您已经投过票了"));
            }
            let votes = current.voters.len() as u64;
            let skipped = votes >= required;
            if skipped {
                let random_index = random_queue_index(state);
                state.advance(now, random_index);
            }
            let result = VoteResult {
                current_votes: votes,
                required_votes: required,
                skipped,
            };
            let mut events = Vec::new();
            if skipped {
                events.push(event(
                    "queue.changed",
                    json!({ "revision": revision, "queue": state.queue() }),
                ));
                events.push(event(
                    "playback.changed",
                    json!({ "revision": revision, "playback": state.playback_at(now) }),
                ));
            }
            Ok((result, events))
        })
        .await
    }

    pub async fn set_play_mode(&self, mode: PlayMode) -> Result<SettingsData, LivehouseError> {
        self.mutate(move |state, revision| {
            state.play_mode = mode;
            let settings = SettingsData { play_mode: mode };
            Ok((
                settings.clone(),
                vec![event(
                    "settings.changed",
                    json!({ "revision": revision, "settings": settings }),
                )],
            ))
        })
        .await
    }

    pub async fn chat(&self, user: User, content: String) -> Result<ChatData, LivehouseError> {
        let content = content.trim().to_owned();
        if content.is_empty() || content.chars().count() > 500 {
            return Err(LivehouseError::invalid("聊天内容长度必须为 1–500 个字符"));
        }
        self.mutate(move |state, revision| {
            let message = ChatData {
                content,
                user,
                sent_at_ms: now_ms(),
            };
            state.chat.push_back(message.clone());
            while state.chat.len() > CHAT_HISTORY_LIMIT {
                state.chat.pop_front();
            }
            Ok((
                message.clone(),
                vec![event(
                    "chat.created",
                    json!({ "revision": revision, "message": message }),
                )],
            ))
        })
        .await
    }

    pub async fn advance_if_expired(&self) -> Result<(), LivehouseError> {
        let expired = {
            let mut preview = self.state.read().await.clone();
            preview.finish_expired(now_ms())
        };
        if !expired || self.is_closed() {
            return Ok(());
        }
        self.mutate(|state, revision| {
            let now = now_ms();
            if !state.finish_expired(now) {
                return Ok(((), Vec::new()));
            }
            Ok((
                (),
                vec![
                    event(
                        "queue.changed",
                        json!({ "revision": revision, "queue": state.queue() }),
                    ),
                    event(
                        "playback.changed",
                        json!({ "revision": revision, "playback": state.playback_at(now) }),
                    ),
                ],
            ))
        })
        .await
    }

    pub async fn is_expired(&self) -> bool {
        if self.state.read().await.ultimate {
            return false;
        }
        self.members.read().await.is_empty()
            && now_ms().saturating_sub(*self.last_active_ms.lock().await) > EMPTY_ROOM_GRACE_MS
    }
}

fn random_queue_index(state: &PersistedRoom) -> Option<usize> {
    if state.play_mode == PlayMode::Random && !state.queue.is_empty() {
        Some(rand::random_range(0..state.queue.len()))
    } else {
        None
    }
}

pub struct RoomManager {
    rooms: RwLock<HashMap<String, Arc<Room>>>,
    store: Arc<dyn StateStore>,
    node_id: String,
    max_members: usize,
}

impl RoomManager {
    pub fn new(store: Arc<dyn StateStore>, node_id: String, max_members: usize) -> Self {
        Self {
            rooms: RwLock::new(HashMap::new()),
            store,
            node_id,
            max_members,
        }
    }

    pub async fn create_room(&self, mut room: PersistedRoom) -> Result<RoomInfo, LivehouseError> {
        room.id = room.id.trim().to_owned();
        room.name = room.name.trim().to_owned();
        if room.id.is_empty()
            || room.id.len() > 128
            || room.name.is_empty()
            || room.name.chars().count() > 80
        {
            return Err(LivehouseError::invalid("房间 ID 或名称无效"));
        }
        if room.password.len() > 256 {
            return Err(LivehouseError::invalid("房间密码过长"));
        }
        if !self
            .store
            .create_room(&room)
            .await
            .map_err(|error| LivehouseError::unavailable(error.to_string()))?
        {
            return self
                .store
                .get_room(&room.id)
                .await
                .map_err(|error| LivehouseError::unavailable(error.to_string()))?
                .map(|room| room.info())
                .ok_or_else(|| LivehouseError::unavailable("房间创建冲突"));
        }
        Ok(room.info())
    }

    pub async fn get(&self, room_id: &str) -> Result<Arc<Room>, LivehouseError> {
        if let Some(room) = self.rooms.read().await.get(room_id) {
            if !room.is_closed() {
                return Ok(room.clone());
            }
        }
        let mut attempts = 0;
        loop {
            let state = self
                .store
                .acquire(room_id, &self.node_id)
                .await
                .map_err(|error| LivehouseError::unavailable(error.to_string()))?;
            if let Some(mut state) = state {
                state.population = 0;
                let room = Arc::new(Room::new(
                    state,
                    self.node_id.clone(),
                    self.store.clone(),
                    self.max_members,
                ));
                let mut rooms = self.rooms.write().await;
                if let Some(existing) = rooms.get(room_id) {
                    if !existing.is_closed() {
                        self.store.release(room_id, &self.node_id).await.ok();
                        return Ok(existing.clone());
                    }
                }
                rooms.insert(room_id.to_owned(), room.clone());
                return Ok(room);
            }
            if self
                .store
                .get_room(room_id)
                .await
                .map_err(|error| LivehouseError::unavailable(error.to_string()))?
                .is_none()
            {
                return Err(LivehouseError::not_found("房间不存在"));
            }
            attempts += 1;
            if attempts >= 24 {
                return Err(LivehouseError::unavailable("房间正在故障接管，请稍后重试"));
            }
            tokio::time::sleep(Duration::from_millis(500)).await;
        }
    }

    pub async fn authenticate(
        &self,
        room_id: &str,
        password: &str,
    ) -> Result<Arc<Room>, LivehouseError> {
        let room = self.get(room_id).await?;
        if !room.password_matches(password).await {
            return Err(LivehouseError::new(ErrorCode::Unauthorized, "密码错误"));
        }
        Ok(room)
    }

    pub async fn list(&self) -> Result<Vec<RoomInfo>, LivehouseError> {
        let stored = self
            .store
            .list_rooms()
            .await
            .map_err(|error| LivehouseError::unavailable(error.to_string()))?;
        let loaded = self.rooms.read().await;
        let mut rooms = Vec::with_capacity(stored.len());
        for state in stored {
            if let Some(room) = loaded.get(&state.id) {
                rooms.push(room.info().await);
            } else {
                rooms.push(state.info());
            }
        }
        rooms.sort_by(|left, right| left.create_time.cmp(&right.create_time));
        Ok(rooms)
    }

    pub async fn tick(&self) {
        let rooms = self
            .rooms
            .read()
            .await
            .values()
            .cloned()
            .collect::<Vec<_>>();
        for room in rooms {
            if let Err(error) = room.advance_if_expired().await {
                tracing::warn!(room_id = room.id, %error, "advance playback");
            }
        }
    }

    pub async fn renew_leases(&self) {
        let rooms = self
            .rooms
            .read()
            .await
            .values()
            .cloned()
            .collect::<Vec<_>>();
        for room in &rooms {
            match self.store.renew(&room.id, &self.node_id).await {
                Ok(true) => {}
                Ok(false) => room.lose_lease(),
                Err(error) => {
                    tracing::error!(room_id = room.id, %error, "renew room lease");
                    room.lose_lease();
                }
            }
        }
        let mut loaded = self.rooms.write().await;
        let closed = loaded
            .iter()
            .filter(|(_, room)| room.is_closed())
            .map(|(id, _)| id.clone())
            .collect::<Vec<_>>();
        for id in closed {
            loaded.remove(&id);
        }
        let mut expired = Vec::new();
        for (id, room) in loaded.iter() {
            if room.is_expired().await {
                expired.push(id.clone());
            }
        }
        for id in expired {
            if let Some(room) = loaded.remove(&id) {
                room.lose_lease();
                let _ = self.store.delete_room(&id, &self.node_id).await;
            }
        }
    }

    pub async fn shutdown(&self) {
        let rooms = self
            .rooms
            .write()
            .await
            .drain()
            .map(|(_, room)| room)
            .collect::<Vec<_>>();
        for room in rooms {
            room.lose_lease();
            let _ = self.store.release(&room.id, &self.node_id).await;
        }
    }
}

pub fn guest_user() -> User {
    User {
        name: format!("游客({})", &Uuid::new_v4().simple().to_string()[..8]),
        email_hash: String::new(),
    }
}

pub fn lag_is_fatal(lagged: u32) -> bool {
    lagged >= MAX_LAGGED_ROOM_EVENTS
}

#[cfg(test)]
mod tests {
    use std::collections::HashSet;

    use crate::{
        model::{PlayableSong, Song, Source},
        store::MemoryStore,
    };

    use super::*;

    fn track(id: &str) -> Track {
        Track {
            playable: PlayableSong {
                song: Song {
                    id: id.to_owned(),
                    source: Source::Qq,
                    name: id.to_owned(),
                    duration_ms: 60_000,
                    ..Song::default()
                },
                url: format!("https://example.invalid/{id}"),
                lyric: String::new(),
            },
            user: User::default(),
            likes: 0,
            voters: HashSet::new(),
        }
    }

    async fn room() -> (Arc<MemoryStore>, Arc<Room>) {
        let store = Arc::new(MemoryStore::default());
        let manager = RoomManager::new(store.clone(), "node-a".to_owned(), 2);
        manager
            .create_room(PersistedRoom::new(
                "room".to_owned(),
                "room".to_owned(),
                String::new(),
                String::new(),
                true,
            ))
            .await
            .unwrap();
        let room = manager.get("room").await.unwrap();
        (store, room)
    }

    #[tokio::test]
    async fn queue_and_playback_are_persisted() {
        let (store, room) = room().await;
        room.enqueue(track("one")).await.unwrap();
        room.enqueue(track("two")).await.unwrap();
        assert_eq!(room.queue().await.items.len(), 1);
        assert_eq!(room.playback().await.song.unwrap().song.id, "one");
        let saved = store.get_room("room").await.unwrap().unwrap();
        assert_eq!(saved.revision, 2);
        assert_eq!(saved.queue.len(), 1);
    }

    #[tokio::test]
    async fn stale_writer_is_fenced() {
        let (store, room) = room().await;
        store.release("room", "node-a").await.unwrap();
        let error = room.enqueue(track("one")).await.unwrap_err();
        assert_eq!(error.code, ErrorCode::LeaseLost);
        assert!(room.is_closed());
        assert!(store
            .get_room("room")
            .await
            .unwrap()
            .unwrap()
            .queue
            .is_empty());
    }

    #[tokio::test]
    async fn rejects_room_overflow() {
        let (_, room) = room().await;
        room.join(1, guest_user()).await.unwrap();
        room.join(2, guest_user()).await.unwrap();
        let error = room.join(3, guest_user()).await.unwrap_err();
        assert_eq!(error.code, ErrorCode::RateLimited);
    }
}
