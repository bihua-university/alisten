use std::{
    collections::{HashSet, VecDeque},
    fmt,
    str::FromStr,
    time::{SystemTime, UNIX_EPOCH},
};

use serde::{Deserialize, Serialize};

use crate::protocol::{ErrorCode, LivehouseError};

pub const CHAT_HISTORY_LIMIT: usize = 100;
pub const START_DELAY_MS: u64 = 200;

pub fn now_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64
}

#[derive(Clone, Copy, Debug, Default, Eq, Hash, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Source {
    #[default]
    Netease,
    Qq,
    Kuwo,
}

impl fmt::Display for Source {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(match self {
            Self::Netease => "netease",
            Self::Qq => "qq",
            Self::Kuwo => "kuwo",
        })
    }
}

impl FromStr for Source {
    type Err = LivehouseError;

    fn from_str(value: &str) -> Result<Self, Self::Err> {
        match value {
            "netease" => Ok(Self::Netease),
            "qq" => Ok(Self::Qq),
            "kuwo" => Ok(Self::Kuwo),
            _ => Err(LivehouseError::new(
                ErrorCode::InvalidArgument,
                "不支持的音乐来源",
            )),
        }
    }
}

#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct User {
    pub name: String,
    #[serde(default)]
    pub email_hash: String,
}

impl User {
    pub fn from_profile(name: String, email: &str, fallback: &str) -> Result<Self, LivehouseError> {
        let name = name.trim();
        if name.is_empty() || name.chars().count() > 80 {
            return Err(LivehouseError::new(
                ErrorCode::InvalidArgument,
                "昵称长度必须为 1–80 个字符",
            ));
        }
        if email.is_empty() {
            return Ok(Self {
                name: format!("{name}({fallback})"),
                email_hash: String::new(),
            });
        }
        if email.len() > 320 {
            return Err(LivehouseError::new(
                ErrorCode::InvalidArgument,
                "邮箱地址过长",
            ));
        }
        Ok(Self {
            name: name.to_owned(),
            email_hash: format!("{:x}", md5::compute(email.trim().to_lowercase())),
        })
    }

    pub fn key(&self) -> String {
        format!("{}\n{}", self.name, self.email_hash)
    }
}

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Song {
    pub id: String,
    pub source: Source,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub artist: String,
    #[serde(default)]
    pub album: String,
    #[serde(default)]
    pub duration_ms: u64,
    #[serde(default)]
    pub picture_url: String,
    #[serde(default)]
    pub web_url: String,
}

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlayableSong {
    #[serde(flatten)]
    pub song: Song,
    #[serde(default)]
    pub url: String,
    #[serde(default)]
    pub lyric: String,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Track {
    pub playable: PlayableSong,
    #[serde(default)]
    pub user: User,
    #[serde(default)]
    pub likes: u64,
    #[serde(default)]
    pub voters: HashSet<String>,
}

impl Track {
    pub fn queue_item(&self) -> QueueItem {
        QueueItem {
            song: self.playable.song.clone(),
            user: self.user.clone(),
            likes: self.likes,
        }
    }

    pub fn key(&self) -> (&str, Source) {
        (&self.playable.song.id, self.playable.song.source)
    }
}

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct QueueItem {
    pub song: Song,
    pub user: User,
    pub likes: u64,
}

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct QueueData {
    pub current: Option<QueueItem>,
    #[serde(default)]
    pub items: Vec<QueueItem>,
    pub online_count: u64,
}

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PlaybackStatus {
    #[default]
    Stopped,
    Playing,
}

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlaybackAnchor {
    #[serde(default)]
    pub status: PlaybackStatus,
    #[serde(default)]
    pub started_at_ms: u64,
    #[serde(default)]
    pub position_ms: u64,
}

impl PlaybackAnchor {
    pub fn position_at(&self, now: u64, duration_ms: u64) -> u64 {
        let elapsed = match self.status {
            PlaybackStatus::Playing => now.saturating_sub(self.started_at_ms),
            PlaybackStatus::Stopped => 0,
        };
        self.position_ms.saturating_add(elapsed).min(duration_ms)
    }
}

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlaybackData {
    pub song: Option<PlayableSong>,
    pub user: Option<User>,
    pub status: PlaybackStatus,
    pub started_at_ms: u64,
    pub position_ms: u64,
    pub server_time_ms: u64,
}

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PlayMode {
    #[default]
    Sequential,
    Random,
}

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SettingsData {
    pub play_mode: PlayMode,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ChatData {
    pub content: String,
    pub user: User,
    pub sent_at_ms: u64,
}

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MembersData {
    #[serde(default)]
    pub users: Vec<User>,
    pub online_count: u64,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RoomInfo {
    pub id: String,
    pub name: String,
    #[serde(rename = "desc")]
    pub description: String,
    pub population: u64,
    pub create_time: u64,
    pub need_pwd: bool,
    pub ultimate: bool,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PersistedRoom {
    pub id: String,
    pub name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub password: String,
    #[serde(default)]
    pub ultimate: bool,
    pub created_at_ms: u64,
    #[serde(default)]
    pub revision: u64,
    #[serde(default)]
    pub population: u64,
    #[serde(default)]
    pub owner: Option<User>,
    #[serde(default)]
    pub queue: VecDeque<Track>,
    #[serde(default)]
    pub current: Option<Track>,
    #[serde(default)]
    pub playback: PlaybackAnchor,
    #[serde(default)]
    pub play_mode: PlayMode,
    #[serde(default)]
    pub chat: VecDeque<ChatData>,
}

impl PersistedRoom {
    pub fn new(
        id: String,
        name: String,
        description: String,
        password: String,
        ultimate: bool,
    ) -> Self {
        Self {
            id,
            name,
            description,
            password,
            ultimate,
            created_at_ms: now_ms(),
            revision: 0,
            population: 0,
            owner: None,
            queue: VecDeque::new(),
            current: None,
            playback: PlaybackAnchor::default(),
            play_mode: PlayMode::Sequential,
            chat: VecDeque::new(),
        }
    }

    pub fn info(&self) -> RoomInfo {
        RoomInfo {
            id: self.id.clone(),
            name: self.name.clone(),
            description: self.description.clone(),
            population: self.population,
            create_time: self.created_at_ms,
            need_pwd: !self.password.is_empty(),
            ultimate: self.ultimate,
        }
    }

    pub fn queue(&self) -> QueueData {
        QueueData {
            current: self.current.as_ref().map(Track::queue_item),
            items: self.queue.iter().map(Track::queue_item).collect(),
            online_count: self.population,
        }
    }

    pub fn playback_at(&self, now: u64) -> PlaybackData {
        let duration = self
            .current
            .as_ref()
            .map(|track| track.playable.song.duration_ms)
            .unwrap_or_default();
        PlaybackData {
            song: self.current.as_ref().map(|track| track.playable.clone()),
            user: self.current.as_ref().map(|track| track.user.clone()),
            status: self.playback.status,
            started_at_ms: self.playback.started_at_ms,
            position_ms: self.playback.position_at(now, duration),
            server_time_ms: now,
        }
    }

    pub fn finish_expired(&mut self, now: u64) -> bool {
        let Some(current) = self.current.as_ref() else {
            return false;
        };
        if self.playback.status != PlaybackStatus::Playing {
            return false;
        }
        let end = self
            .playback
            .started_at_ms
            .saturating_add(current.playable.song.duration_ms)
            .saturating_sub(self.playback.position_ms);
        if now < end {
            return false;
        }
        self.advance(now, None);
        true
    }

    pub fn advance(&mut self, now: u64, random_index: Option<usize>) {
        self.vote_clear();
        if self.queue.is_empty() {
            self.current = None;
            self.playback = PlaybackAnchor::default();
            return;
        }
        let index = random_index
            .filter(|index| *index < self.queue.len())
            .unwrap_or_default();
        let next = self.queue.remove(index).expect("queue index checked");
        self.playback = PlaybackAnchor {
            status: PlaybackStatus::Playing,
            started_at_ms: now.saturating_add(START_DELAY_MS),
            position_ms: 0,
        };
        self.current = Some(next);
    }

    fn vote_clear(&mut self) {
        if let Some(current) = self.current.as_mut() {
            current.voters.clear();
        }
    }
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RoomSnapshot {
    pub revision: u64,
    pub room: RoomInfo,
    pub queue: QueueData,
    pub playback: PlaybackData,
    pub members: MembersData,
    pub settings: SettingsData,
    pub chat: Vec<ChatData>,
    pub server_time_ms: u64,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PickResult {
    pub id: String,
    pub name: String,
    pub artist: String,
    pub source: Source,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SongRef {
    pub id: String,
    pub source: Source,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LikeResult {
    pub id: String,
    pub source: Source,
    pub name: String,
    pub likes: u64,
}

#[derive(Clone, Debug, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct VoteResult {
    pub current_votes: u64,
    pub required_votes: u64,
    pub skipped: bool,
}

#[cfg(test)]
mod tests {
    use super::*;

    fn track(id: &str, duration_ms: u64) -> Track {
        Track {
            playable: PlayableSong {
                song: Song {
                    id: id.to_owned(),
                    duration_ms,
                    ..Song::default()
                },
                ..PlayableSong::default()
            },
            user: User::default(),
            likes: 0,
            voters: HashSet::new(),
        }
    }

    #[test]
    fn derives_progress_from_anchor_and_clamps_to_duration() {
        let room = PersistedRoom {
            current: Some(track("one", 10_000)),
            playback: PlaybackAnchor {
                status: PlaybackStatus::Playing,
                started_at_ms: 1_000,
                position_ms: 500,
            },
            ..PersistedRoom::new(
                "room".into(),
                "room".into(),
                String::new(),
                String::new(),
                false,
            )
        };
        assert_eq!(room.playback_at(2_000).position_ms, 1_500);
        assert_eq!(room.playback_at(50_000).position_ms, 10_000);
    }

    #[test]
    fn expired_playback_restores_next_track() {
        let mut room = PersistedRoom::new(
            "room".into(),
            "room".into(),
            String::new(),
            String::new(),
            false,
        );
        room.current = Some(track("one", 1_000));
        room.queue.push_back(track("two", 2_000));
        room.playback = PlaybackAnchor {
            status: PlaybackStatus::Playing,
            started_at_ms: 100,
            position_ms: 0,
        };
        assert!(room.finish_expired(1_200));
        assert_eq!(room.current.unwrap().playable.song.id, "two");
        assert_eq!(room.playback.started_at_ms, 1_400);
        assert!(room.queue.is_empty());
    }
}
