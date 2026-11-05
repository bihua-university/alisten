use std::{convert::Infallible, sync::Arc};

use bytes::Bytes;
use futures_util::{SinkExt, StreamExt};
use http::{HeaderMap, Method, Request, Response, StatusCode};
use http_body_util::{BodyExt, Full, Limited};
use hyper::{body::Incoming as HyperIncoming, upgrade::Upgraded};
use hyper_util::rt::TokioIo;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use tokio::sync::{mpsc, RwLock, Semaphore};
use tokio_tungstenite::{
    tungstenite::{handshake::derive_accept_key, protocol::Role, Message},
    WebSocketStream,
};
use tower::service_fn;

use crate::{
    model::{now_ms, PersistedRoom, PlayMode, Track, User},
    protocol::{
        event, json_response, sync_required, ErrorCode, HttpResponse, Incoming, LivehouseError,
    },
    room::{guest_user, lag_is_fatal, Room, RoomManager},
    store::RedisStore,
};

type HyperRequest = Request<HyperIncoming>;
type Socket = WebSocketStream<TokioIo<Upgraded>>;

#[derive(Clone)]
pub struct AppState {
    pub rooms: Arc<RoomManager>,
    pub internal_token: Option<Arc<str>>,
    pub node_id: Arc<str>,
    pub store: Arc<RedisStore>,
    pub http: reqwest::Client,
}

pub fn router(
    state: AppState,
) -> impl tower::Service<
    HyperRequest,
    Response = HttpResponse,
    Error = Infallible,
    Future = impl std::future::Future<Output = Result<HttpResponse, Infallible>> + Send,
> + Clone {
    service_fn(move |request: HyperRequest| {
        let state = state.clone();
        async move {
            let result = match (request.method(), request.uri().path()) {
                (&Method::GET, "/healthz") => Ok(json_response(StatusCode::OK, "ok")),
                (&Method::GET, "/ws") => websocket(state, request).await,
                (&Method::POST, "/rpc") => rpc(state, request).await,
                _ => Err(LivehouseError::not_found("接口不存在")),
            };
            Ok::<_, Infallible>(match result {
                Ok(response) => response,
                Err(error) => error.into_response(),
            })
        }
    })
}

#[derive(Default)]
struct ConnectQuery {
    room_id: String,
    password: String,
}

fn connect_query(request: &HyperRequest) -> ConnectQuery {
    let mut query = ConnectQuery::default();
    for (key, value) in
        url::form_urlencoded::parse(request.uri().query().unwrap_or_default().as_bytes())
    {
        match key.as_ref() {
            "roomId" | "houseId" => query.room_id = value.into_owned(),
            "password" | "housePwd" => query.password = value.into_owned(),
            _ => {}
        }
    }
    query
}

async fn websocket(state: AppState, request: HyperRequest) -> Result<HttpResponse, LivehouseError> {
    let query = connect_query(&request);
    let (owner, _) = state
        .store
        .assigned_node(&query.room_id, &state.node_id)
        .await
        .map_err(|error| LivehouseError::unavailable(error.to_string()))?;
    if owner != state.node_id.as_ref() {
        return Err(LivehouseError::unavailable("房间归属其他 livehouse 节点"));
    }
    let room = state
        .rooms
        .authenticate(&query.room_id, &query.password)
        .await?;
    let key = request
        .headers()
        .get(http::header::SEC_WEBSOCKET_KEY)
        .and_then(|value| value.to_str().ok())
        .ok_or_else(|| LivehouseError::invalid("缺少 WebSocket key"))?
        .to_owned();
    let response = Response::builder()
        .status(StatusCode::SWITCHING_PROTOCOLS)
        .header(http::header::UPGRADE, "websocket")
        .header(http::header::CONNECTION, "Upgrade")
        .header(
            http::header::SEC_WEBSOCKET_ACCEPT,
            derive_accept_key(key.as_bytes()),
        )
        .body(Full::new(Bytes::new()))
        .expect("valid WebSocket upgrade response");
    tokio::spawn(async move {
        match hyper::upgrade::on(request).await {
            Ok(upgraded) => {
                let socket =
                    WebSocketStream::from_raw_socket(TokioIo::new(upgraded), Role::Server, None)
                        .await;
                serve_websocket(socket, room).await;
            }
            Err(error) => tracing::warn!(%error, "WebSocket upgrade failed"),
        }
    });
    Ok(response)
}

async fn serve_websocket(socket: Socket, room: Arc<Room>) {
    let member_id = rand::random::<u64>();
    let user = guest_user();
    let (mut events, snapshot) = match room.join(member_id, user.clone()).await {
        Ok(joined) => joined,
        Err(error) => {
            let (mut sender, _) = socket.split();
            let _ = sender
                .send(Message::Text(
                    event("protocol.error", error).to_string().into(),
                ))
                .await;
            return;
        }
    };
    let (mut sender, mut receiver) = socket.split();
    if sender
        .send(Message::Text(
            event("room.snapshot", snapshot).to_string().into(),
        ))
        .await
        .is_err()
    {
        room.leave(member_id).await;
        return;
    }

    let (outbound, mut outgoing) = mpsc::channel::<Value>(128);
    let writer = tokio::spawn(async move {
        while let Some(message) = outgoing.recv().await {
            let is_close = message["type"] == "room.closed";
            if sender
                .send(Message::Text(message.to_string().into()))
                .await
                .is_err()
            {
                break;
            }
            if is_close {
                let _ = sender.send(Message::Close(None)).await;
                break;
            }
        }
    });

    let event_output = outbound.clone();
    let event_task = tokio::spawn(async move {
        let mut lagged = 0;
        loop {
            match events.recv().await {
                Ok(raw) => {
                    let Ok(message) = serde_json::from_str::<Value>(&raw) else {
                        continue;
                    };
                    let close = message["type"] == "room.closed";
                    if event_output.try_send(message).is_err() {
                        break;
                    }
                    if close {
                        break;
                    }
                }
                Err(tokio::sync::broadcast::error::RecvError::Lagged(_)) => {
                    lagged += 1;
                    if lag_is_fatal(lagged)
                        || event_output
                            .try_send(sync_required("broadcast_lagged"))
                            .is_err()
                    {
                        break;
                    }
                }
                Err(tokio::sync::broadcast::error::RecvError::Closed) => break,
            }
        }
    });

    let current_user = Arc::new(RwLock::new(user));
    let pending = Arc::new(Semaphore::new(16));
    while let Some(message) = receiver.next().await {
        let raw = match message {
            Ok(Message::Text(raw)) if raw.len() <= 64 * 1024 => raw,
            Ok(Message::Ping(_)) | Ok(Message::Pong(_)) => continue,
            Ok(Message::Close(_)) => break,
            Ok(_) => {
                let _ = outbound.try_send(event(
                    "protocol.error",
                    LivehouseError::invalid("仅接受不超过 64 KiB 的 UTF-8 JSON 文本消息"),
                ));
                continue;
            }
            Err(_) => break,
        };
        let message = match serde_json::from_str::<Incoming>(&raw) {
            Ok(message) => message,
            Err(_) => {
                let _ = outbound.try_send(event(
                    "protocol.error",
                    LivehouseError::new(ErrorCode::InvalidMessage, "消息不是有效的协议请求"),
                ));
                continue;
            }
        };
        if let Err(error) = message.validate() {
            let _ = outbound.try_send(message.failure(&error));
            continue;
        }
        let Ok(permit) = pending.clone().try_acquire_owned() else {
            let _ = outbound.try_send(message.failure(&LivehouseError::new(
                ErrorCode::RateLimited,
                "未完成请求过多，请稍后再试",
            )));
            continue;
        };
        let output = outbound.clone();
        let room = room.clone();
        let user = current_user.clone();
        tokio::spawn(async move {
            let _permit = permit;
            let response = match execute(
                &room,
                &user,
                Some(member_id),
                &message.message_type,
                message.data.clone(),
            )
            .await
            {
                Ok(data) => message.response(data),
                Err(error) => message.failure(&error),
            };
            let _ = output.send(response).await;
        });
    }
    drop(outbound);
    event_task.abort();
    room.leave(member_id).await;
    writer.abort();
}

async fn execute(
    room: &Arc<Room>,
    current_user: &Arc<RwLock<User>>,
    member_id: Option<u64>,
    operation: &str,
    data: Value,
) -> Result<Value, LivehouseError> {
    fn decode<T: serde::de::DeserializeOwned>(data: Value) -> Result<T, LivehouseError> {
        serde_json::from_value(data).map_err(|_| LivehouseError::invalid("请求数据格式错误"))
    }

    match operation {
        "clock.ping" => {
            #[derive(Deserialize)]
            #[serde(rename_all = "camelCase")]
            struct Ping {
                client_time_ms: u64,
            }
            let ping: Ping = decode(data)?;
            Ok(json!({ "clientTimeMs": ping.client_time_ms, "serverTimeMs": now_ms() }))
        }
        "room.sync" => Ok(json!(room.snapshot().await)),
        "chat.send" => {
            #[derive(Deserialize)]
            struct Chat {
                content: String,
            }
            let chat: Chat = decode(data)?;
            room.chat(current_user.read().await.clone(), chat.content)
                .await?;
            Ok(json!({}))
        }
        "member.update" => {
            #[derive(Deserialize)]
            #[serde(rename_all = "camelCase")]
            struct Update {
                name: String,
                #[serde(default)]
                email: String,
                #[serde(default)]
                client_time_ms: u64,
            }
            let member_id =
                member_id.ok_or_else(|| LivehouseError::invalid("仅 WebSocket 成员可更新资料"))?;
            let update: Update = decode(data)?;
            let user = User::from_profile(update.name, &update.email, "已连接")?;
            room.update_user(member_id, user.clone()).await?;
            *current_user.write().await = user.clone();
            Ok(json!({
                "user": user,
                "clientTimeMs": update.client_time_ms,
                "serverTimeMs": now_ms(),
            }))
        }
        "queue.enqueue" => {
            if member_id.is_some() {
                return Err(LivehouseError::new(
                    ErrorCode::UnknownType,
                    "点歌请使用 livehouse-api HTTP API",
                ));
            }
            let track: Track = decode(data)?;
            if track.playable.song.id.trim().is_empty() || track.playable.song.duration_ms == 0 {
                return Err(LivehouseError::invalid("准备好的歌曲数据不完整"));
            }
            let result = crate::model::PickResult {
                id: track.playable.song.id.clone(),
                name: track.playable.song.name.clone(),
                artist: if track.playable.song.artist.is_empty() {
                    "unknown".to_owned()
                } else {
                    track.playable.song.artist.clone()
                },
                source: track.playable.song.source,
            };
            room.enqueue(track).await?;
            Ok(json!(result))
        }
        "queue.remove" => {
            let removed = room.remove(decode(data)?).await?;
            if member_id.is_some() {
                room.chat(
                    current_user.read().await.clone(),
                    format!("删除音乐 {}", removed.playable.song.name),
                )
                .await?;
            }
            Ok(json!({
                "id": removed.playable.song.id,
                "source": removed.playable.song.source,
                "name": removed.playable.song.name,
            }))
        }
        "queue.like" => Ok(json!(room.like(decode(data)?).await?)),
        "queue.get" => Ok(json!(room.queue().await)),
        "playback.skipVote" => Ok(json!(room.vote(current_user.read().await.clone()).await?)),
        "playback.get" => Ok(json!(room.playback().await)),
        "settings.get" => Ok(json!(room.settings().await)),
        "settings.set" => {
            #[derive(Deserialize)]
            #[serde(rename_all = "camelCase")]
            struct Settings {
                play_mode: Option<PlayMode>,
            }
            let settings: Settings = decode(data)?;
            if let Some(mode) = settings.play_mode {
                Ok(json!(room.set_play_mode(mode).await?))
            } else {
                Ok(json!(room.settings().await))
            }
        }
        "members.get" => Ok(json!(room.members().await)),
        _ => Err(LivehouseError::new(
            ErrorCode::UnknownType,
            "未知或不适于房间通道的操作类型",
        )),
    }
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RpcRequest {
    #[serde(rename = "type")]
    operation: String,
    #[serde(default)]
    room_id: String,
    #[serde(default)]
    password: String,
    #[serde(default = "empty_data")]
    data: Value,
}

fn empty_data() -> Value {
    json!({})
}

#[derive(Deserialize)]
struct CreateRoom {
    #[serde(default)]
    id: Option<String>,
    name: String,
    #[serde(default, alias = "description")]
    desc: String,
    #[serde(default)]
    password: String,
    #[serde(default)]
    ultimate: bool,
}

async fn rpc(state: AppState, request: HyperRequest) -> Result<HttpResponse, LivehouseError> {
    let (parts, body) = request.into_parts();
    check_internal_token(&state, &parts.headers)?;
    let body = Limited::new(body, 1024 * 1024)
        .collect()
        .await
        .map_err(|_| LivehouseError::invalid("RPC 请求体过大或无法读取"))?
        .to_bytes();
    let request: RpcRequest = serde_json::from_slice(&body)
        .map_err(|_| LivehouseError::invalid("RPC 请求数据格式错误"))?;
    if !matches!(
        request.operation.as_str(),
        "room.create" | "room.ensure" | "room.list"
    ) {
        let (owner, address) = state
            .store
            .assigned_node(&request.room_id, &state.node_id)
            .await
            .map_err(|error| LivehouseError::unavailable(error.to_string()))?;
        if owner != state.node_id.as_ref() {
            let mut outgoing = state
                .http
                .post(format!("http://{address}/rpc"))
                .json(&request);
            if let Some(token) = &state.internal_token {
                outgoing = outgoing.bearer_auth(token);
            }
            let response = outgoing.send().await.map_err(|error| {
                LivehouseError::unavailable(format!("RPC 节点转发失败: {error}"))
            })?;
            let status =
                StatusCode::from_u16(response.status().as_u16()).unwrap_or(StatusCode::BAD_GATEWAY);
            let body = response
                .bytes()
                .await
                .map_err(|error| LivehouseError::unavailable(error.to_string()))?;
            return Ok(Response::builder()
                .status(status)
                .header(http::header::CONTENT_TYPE, "application/json")
                .body(Full::new(body))
                .expect("valid RPC forwarding response"));
        }
    }
    let data = match request.operation.as_str() {
        "room.create" | "room.ensure" => {
            let mut create: CreateRoom = decode_rpc(request.data)?;
            if request.operation == "room.ensure" {
                if create.id.is_none() {
                    return Err(LivehouseError::invalid("持久化房间必须指定 ID"));
                }
                create.ultimate = true;
            }
            let room = state
                .rooms
                .create_room(PersistedRoom::new(
                    create
                        .id
                        .unwrap_or_else(|| uuid::Uuid::new_v4().to_string()),
                    create.name,
                    create.desc,
                    create.password,
                    create.ultimate,
                ))
                .await?;
            json!(room)
        }
        "room.list" => json!(state.rooms.list().await?),
        "room.enter" => {
            let room = state
                .rooms
                .authenticate(&request.room_id, &request.password)
                .await?;
            json!(room.info().await)
        }
        _ => {
            let room = state
                .rooms
                .authenticate(&request.room_id, &request.password)
                .await?;
            let user = match request.data.get("user").cloned() {
                Some(value) => {
                    #[derive(Deserialize)]
                    #[serde(rename_all = "camelCase")]
                    struct LegacyUser {
                        name: String,
                        #[serde(default)]
                        email: String,
                        #[serde(default)]
                        email_hash: String,
                    }
                    let user: LegacyUser = decode_rpc(value)?;
                    if user.email_hash.is_empty() && !user.email.is_empty() {
                        User::from_profile(user.name, &user.email, "HTTP API")?
                    } else {
                        User {
                            name: user.name,
                            email_hash: user.email_hash,
                        }
                    }
                }
                None => User {
                    name: "HTTP API".to_owned(),
                    email_hash: String::new(),
                },
            };
            execute(
                &room,
                &Arc::new(RwLock::new(user)),
                None,
                &request.operation,
                request.data,
            )
            .await?
        }
    };
    Ok(json_response(
        StatusCode::OK,
        json!({ "version": 2, "data": data }),
    ))
}

fn decode_rpc<T: serde::de::DeserializeOwned>(data: Value) -> Result<T, LivehouseError> {
    serde_json::from_value(data).map_err(|_| LivehouseError::invalid("RPC 数据格式错误"))
}

fn check_internal_token(state: &AppState, headers: &HeaderMap) -> Result<(), LivehouseError> {
    let Some(expected) = &state.internal_token else {
        return Ok(());
    };
    let authorized = headers
        .get(http::header::AUTHORIZATION)
        .and_then(|value| value.to_str().ok())
        .and_then(|value| value.strip_prefix("Bearer "))
        .is_some_and(|token| token == expected.as_ref());
    if authorized {
        Ok(())
    } else {
        Err(LivehouseError::new(
            ErrorCode::Unauthorized,
            "内部 API 鉴权失败",
        ))
    }
}
