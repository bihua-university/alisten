use std::{
    sync::Mutex,
    time::{Duration, Instant},
};

use anyhow::{anyhow, bail, Context, Result};
use log::warn;
use reqwest::{Method, Response, StatusCode};
use serde::Deserialize;
use tokio::sync::Mutex as AsyncMutex;
use url::Url;

const DEFAULT_GROUP: &str = "DEFAULT_GROUP";
const MAX_RESPONSE_BYTES: usize = 1024 * 1024;
const DISCOVERY_TTL: Duration = Duration::from_secs(5);
const STALE_TTL: Duration = Duration::from_secs(30);

pub struct NacosClient {
    base_url: Url,
    namespace: Option<String>,
    username: Option<String>,
    password: String,
    http: reqwest::Client,
    token: AsyncMutex<Option<Token>>,
    cache: Mutex<Option<CachedInstance>>,
    refresh: AsyncMutex<()>,
}

struct Token {
    value: String,
    refresh_at: Instant,
}

#[derive(Clone)]
struct CachedInstance {
    address: String,
    fresh_until: Instant,
    stale_until: Instant,
}

#[derive(Deserialize)]
struct InstanceList {
    #[serde(default)]
    hosts: Vec<Instance>,
}

#[derive(Deserialize)]
struct Instance {
    ip: String,
    port: u16,
    #[serde(default)]
    healthy: bool,
    #[serde(default)]
    enabled: bool,
}

#[derive(Deserialize)]
struct LoginReply {
    #[serde(rename = "accessToken")]
    access_token: String,
    #[serde(rename = "tokenTtl")]
    token_ttl: Option<u64>,
}

impl NacosClient {
    pub fn new(server_address: &str) -> Result<Self> {
        let server_address = server_address.trim();
        if server_address.is_empty() {
            bail!("NACOS_SERVER_ADDR is required");
        }
        let server_address = if server_address.contains("://") {
            server_address.to_owned()
        } else {
            format!("http://{server_address}")
        };
        let mut base_url = Url::parse(&server_address).context("parse Nacos server address")?;
        if !matches!(base_url.scheme(), "http" | "https") || base_url.host_str().is_none() {
            bail!("Nacos server address must be an HTTP(S) URL");
        }
        let namespace = base_url
            .query_pairs()
            .find(|(key, _)| key == "namespace")
            .map(|(_, value)| value.into_owned())
            .filter(|value| !value.is_empty() && value != "public");
        let username = if base_url.username().is_empty() {
            None
        } else {
            Some(base_url.username().to_owned())
        };
        let password = base_url.password().unwrap_or_default().to_owned();
        base_url
            .set_username("")
            .map_err(|_| anyhow!("clear Nacos URL username"))?;
        base_url
            .set_password(None)
            .map_err(|_| anyhow!("clear Nacos URL password"))?;
        base_url.set_query(None);
        base_url.set_fragment(None);
        let path = base_url.path().trim_end_matches('/');
        if path.is_empty() {
            base_url.set_path("/nacos");
        }

        Ok(Self {
            base_url,
            namespace,
            username,
            password,
            http: reqwest::Client::builder()
                .timeout(Duration::from_secs(15))
                .build()?,
            token: AsyncMutex::new(None),
            cache: Mutex::new(None),
            refresh: AsyncMutex::new(()),
        })
    }

    pub async fn discover(&self, service_name: &str) -> Result<String> {
        if let Some(address) = self.cached(true) {
            return Ok(address);
        }
        let _refresh = self.refresh.lock().await;
        if let Some(address) = self.cached(true) {
            return Ok(address);
        }
        match self.fetch(service_name).await {
            Ok(address) => {
                let now = Instant::now();
                *self.lock_cache() = Some(CachedInstance {
                    address: address.clone(),
                    fresh_until: now + DISCOVERY_TTL,
                    stale_until: now + STALE_TTL,
                });
                Ok(address)
            }
            Err(error) => {
                if let Some(address) = self.cached(false) {
                    warn!("Nacos discovery failed; using stale room-service {address}: {error:#}");
                    return Ok(address);
                }
                Err(error)
            }
        }
    }

    pub fn invalidate(&self) {
        *self.lock_cache() = None;
    }

    fn cached(&self, fresh_only: bool) -> Option<String> {
        let cache = self.lock_cache();
        let cached = cache.as_ref()?;
        let now = Instant::now();
        if now < cached.fresh_until || (!fresh_only && now < cached.stale_until) {
            Some(cached.address.clone())
        } else {
            None
        }
    }

    fn lock_cache(&self) -> std::sync::MutexGuard<'_, Option<CachedInstance>> {
        self.cache
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
    }

    async fn fetch(&self, service_name: &str) -> Result<String> {
        let query = vec![
            ("serviceName", service_name),
            ("groupName", DEFAULT_GROUP),
            ("healthyOnly", "true"),
        ];
        let body = self
            .request(Method::GET, "/v1/ns/instance/list", &query, None)
            .await?;
        let instances: InstanceList =
            serde_json::from_slice(&body).context("decode Nacos instances")?;
        instances
            .hosts
            .into_iter()
            .find(|instance| {
                instance.healthy && instance.enabled && !instance.ip.is_empty() && instance.port > 0
            })
            .map(|instance| {
                if instance.ip.contains(':') {
                    format!("[{}]:{}", instance.ip, instance.port)
                } else {
                    format!("{}:{}", instance.ip, instance.port)
                }
            })
            .ok_or_else(|| anyhow!("Nacos service {service_name} has no healthy instances"))
    }

    async fn request(
        &self,
        method: Method,
        path: &str,
        query: &[(&str, &str)],
        form: Option<&[(&str, &str)]>,
    ) -> Result<Vec<u8>> {
        let token = self.request_token(false).await?;
        let response = self
            .send(&method, path, query, form, token.as_deref())
            .await?;
        let response = if self.username.is_some()
            && matches!(
                response.status(),
                StatusCode::UNAUTHORIZED | StatusCode::FORBIDDEN
            ) {
            let token = self.request_token(true).await?;
            self.send(&method, path, query, form, token.as_deref())
                .await?
        } else {
            response
        };
        read_response(response).await
    }

    async fn send(
        &self,
        method: &Method,
        path: &str,
        query: &[(&str, &str)],
        form: Option<&[(&str, &str)]>,
        token: Option<&str>,
    ) -> Result<Response> {
        let mut url = self.base_url.clone();
        let full_path = format!("{}{}", self.base_url.path().trim_end_matches('/'), path);
        url.set_path(&full_path);
        {
            let mut pairs = url.query_pairs_mut();
            for (key, value) in query {
                pairs.append_pair(key, value);
            }
            if path.starts_with("/v1/ns/") {
                if let Some(namespace) = &self.namespace {
                    pairs.append_pair("namespaceId", namespace);
                }
            }
            if let Some(token) = token {
                pairs.append_pair("accessToken", token);
            }
        }
        let mut request = self.http.request(method.clone(), url);
        if let Some(form) = form {
            request = request.form(form);
        }
        request.send().await.context("request Nacos")
    }

    async fn request_token(&self, force: bool) -> Result<Option<String>> {
        if self.username.is_none() {
            return Ok(None);
        }
        let mut token = self.token.lock().await;
        if !force {
            if let Some(token) = token.as_ref() {
                if Instant::now() < token.refresh_at {
                    return Ok(Some(token.value.clone()));
                }
            }
        }
        let username = self.username.as_deref().unwrap_or_default();
        let form = [("username", username), ("password", self.password.as_str())];
        let response = self
            .send(&Method::POST, "/v1/auth/login", &[], Some(&form), None)
            .await?;
        let body = read_response(response).await?;
        let reply: LoginReply =
            serde_json::from_slice(&body).context("decode Nacos login response")?;
        if reply.access_token.is_empty() {
            bail!("Nacos login response has no access token");
        }
        let lifetime = Duration::from_secs(reply.token_ttl.filter(|ttl| *ttl > 0).unwrap_or(3600));
        let refresh_after = lifetime
            .checked_sub(Duration::from_secs(60))
            .unwrap_or(lifetime / 2);
        *token = Some(Token {
            value: reply.access_token.clone(),
            refresh_at: Instant::now() + refresh_after,
        });
        Ok(Some(reply.access_token))
    }
}

async fn read_response(response: Response) -> Result<Vec<u8>> {
    let status = response.status();
    let mut response = response;
    let mut body = Vec::new();
    while let Some(chunk) = response.chunk().await? {
        if body.len() + chunk.len() > MAX_RESPONSE_BYTES {
            bail!("Nacos response exceeds {MAX_RESPONSE_BYTES} bytes");
        }
        body.extend_from_slice(&chunk);
    }
    if !status.is_success() {
        let summary = String::from_utf8_lossy(&body);
        let summary = summary.trim();
        bail!(
            "Nacos request failed with HTTP {status}: {}",
            if summary.is_empty() {
                "empty response"
            } else {
                summary
            }
        );
    }
    Ok(body)
}

#[cfg(test)]
mod tests {
    use std::sync::{
        atomic::{AtomicUsize, Ordering},
        Arc, Mutex,
    };

    use tokio::{
        io::{AsyncReadExt, AsyncWriteExt},
        net::TcpListener,
    };

    use super::*;

    #[test]
    fn validates_server_address() {
        assert!(NacosClient::new("").is_err());
        assert!(NacosClient::new("ftp://localhost").is_err());
        assert!(NacosClient::new("localhost:8848").is_ok());
    }

    #[tokio::test]
    async fn authenticates_discovers_and_invalidates_cache() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = listener.local_addr().unwrap();
        let logins = Arc::new(AtomicUsize::new(0));
        let requests = Arc::new(Mutex::new(Vec::new()));
        let server_logins = Arc::clone(&logins);
        let server_requests = Arc::clone(&requests);
        let server = tokio::spawn(async move {
            loop {
                let (mut stream, _) = listener.accept().await.unwrap();
                let mut data = Vec::new();
                let mut buffer = [0; 4096];
                let size = stream.read(&mut buffer).await.unwrap();
                data.extend_from_slice(&buffer[..size]);
                let request = String::from_utf8_lossy(&data).into_owned();
                let path = request.lines().next().unwrap().to_owned();
                server_requests.lock().unwrap().push(path.clone());
                let (status, body) = if path.starts_with("POST /nacos/v1/auth/login") {
                    let login = server_logins.fetch_add(1, Ordering::SeqCst) + 1;
                    (
                        200,
                        format!(r#"{{"accessToken":"token-{login}","tokenTtl":3600}}"#),
                    )
                } else if path.contains("accessToken=token-1") {
                    (403, "expired".to_owned())
                } else {
                    (200, r#"{"hosts":[{"ip":"127.0.0.1","port":8080,"healthy":false,"enabled":true},{"ip":"127.0.0.1","port":8081,"healthy":true,"enabled":true}]}"#.to_owned())
                };
                let response = format!(
                    "HTTP/1.1 {status} OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                );
                stream.write_all(response.as_bytes()).await.unwrap();
            }
        });

        let client = NacosClient::new(&format!(
            "http://user:pass@{address}/nacos?namespace=development"
        ))
        .unwrap();
        assert_eq!(
            client.discover("room-service").await.unwrap(),
            "127.0.0.1:8081"
        );
        assert_eq!(
            client.discover("room-service").await.unwrap(),
            "127.0.0.1:8081"
        );
        assert_eq!(requests.lock().unwrap().len(), 4);
        client.invalidate();
        assert_eq!(
            client.discover("room-service").await.unwrap(),
            "127.0.0.1:8081"
        );
        assert_eq!(requests.lock().unwrap().len(), 5);
        assert!(requests.lock().unwrap().iter().all(|request| {
            request.starts_with("POST /nacos/v1/auth/login")
                || request.contains("namespaceId=development")
        }));
        assert_eq!(logins.load(Ordering::SeqCst), 2);
        server.abort();
    }
}
