use bytes::Bytes;
use http::{Response, StatusCode};
use http_body_util::Full;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use thiserror::Error;

pub const VERSION: u16 = 2;

#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum ErrorCode {
    InvalidMessage,
    UnsupportedVersion,
    UnknownType,
    InvalidArgument,
    NotFound,
    Unauthorized,
    RateLimited,
    Conflict,
    ProviderError,
    Unavailable,
    LeaseLost,
    InternalError,
}

#[derive(Clone, Debug, Error, Serialize)]
#[error("{message}")]
pub struct LivehouseError {
    pub code: ErrorCode,
    pub message: String,
}

impl LivehouseError {
    pub fn new(code: ErrorCode, message: impl Into<String>) -> Self {
        Self {
            code,
            message: message.into(),
        }
    }

    pub fn invalid(message: impl Into<String>) -> Self {
        Self::new(ErrorCode::InvalidArgument, message)
    }

    pub fn not_found(message: impl Into<String>) -> Self {
        Self::new(ErrorCode::NotFound, message)
    }

    pub fn conflict(message: impl Into<String>) -> Self {
        Self::new(ErrorCode::Conflict, message)
    }

    pub fn unavailable(message: impl Into<String>) -> Self {
        Self::new(ErrorCode::Unavailable, message)
    }

    pub fn status_code(&self) -> StatusCode {
        match self.code {
            ErrorCode::NotFound => StatusCode::NOT_FOUND,
            ErrorCode::Unauthorized => StatusCode::UNAUTHORIZED,
            ErrorCode::RateLimited => StatusCode::TOO_MANY_REQUESTS,
            ErrorCode::Conflict => StatusCode::CONFLICT,
            ErrorCode::ProviderError | ErrorCode::Unavailable | ErrorCode::LeaseLost => {
                StatusCode::SERVICE_UNAVAILABLE
            }
            ErrorCode::InternalError => StatusCode::INTERNAL_SERVER_ERROR,
            _ => StatusCode::BAD_REQUEST,
        }
    }
}

#[derive(Serialize)]
struct ErrorBody {
    error: String,
}

pub type HttpResponse = Response<Full<Bytes>>;

pub fn json_response<T: Serialize>(status: StatusCode, value: T) -> HttpResponse {
    let body = serde_json::to_vec(&value).unwrap_or_else(|_| b"{}".to_vec());
    Response::builder()
        .status(status)
        .header(http::header::CONTENT_TYPE, "application/json")
        .body(Full::new(Bytes::from(body)))
        .expect("valid HTTP response")
}

impl LivehouseError {
    pub fn into_response(self) -> HttpResponse {
        json_response(
            self.status_code(),
            ErrorBody {
                error: self.message,
            },
        )
    }
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Incoming {
    pub version: u16,
    pub kind: String,
    #[serde(rename = "type")]
    pub message_type: String,
    #[serde(default)]
    pub request_id: String,
    #[serde(default = "empty_object")]
    pub data: Value,
}

fn empty_object() -> Value {
    json!({})
}

impl Incoming {
    pub fn validate(&self) -> Result<(), LivehouseError> {
        if self.version != VERSION {
            return Err(LivehouseError::new(
                ErrorCode::UnsupportedVersion,
                "仅支持 WebSocket 协议 v2",
            ));
        }
        if self.kind != "request" || self.request_id.is_empty() || self.request_id.len() > 128 {
            return Err(LivehouseError::new(
                ErrorCode::InvalidMessage,
                "请求必须包含有效 requestId",
            ));
        }
        Ok(())
    }

    pub fn response<T: Serialize>(&self, data: T) -> Value {
        envelope("response", &self.message_type, Some(&self.request_id), data)
    }

    pub fn failure(&self, error: &LivehouseError) -> Value {
        json!({
            "version": VERSION,
            "kind": "response",
            "type": self.message_type,
            "requestId": self.request_id,
            "data": {},
            "error": error,
        })
    }
}

pub fn event<T: Serialize>(message_type: &str, data: T) -> Value {
    envelope("event", message_type, None, data)
}

fn envelope<T: Serialize>(
    kind: &str,
    message_type: &str,
    request_id: Option<&str>,
    data: T,
) -> Value {
    let mut message = json!({
        "version": VERSION,
        "kind": kind,
        "type": message_type,
        "data": data,
    });
    if let Some(request_id) = request_id {
        message["requestId"] = json!(request_id);
    }
    message
}

pub fn sync_required(reason: &str) -> Value {
    event("sync.required", json!({ "reason": reason }))
}
