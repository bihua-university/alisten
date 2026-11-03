mod nacos;
mod tls;

use anyhow::{bail, Result};
use async_trait::async_trait;
use nacos::NacosClient;
use pingora::{
    http::RequestHeader,
    listeners::tls::TlsSettings,
    proxy::{http_proxy_service, ProxyHttp, Session},
    server::Server,
    services::background::background_service,
    upstreams::peer::HttpPeer,
    Error, ErrorType, Result as PingoraResult,
};
use tls::CertificateResolver;

struct Gateway {
    nacos: NacosClient,
    service_name: String,
}

#[async_trait]
impl ProxyHttp for Gateway {
    type CTX = ();

    fn new_ctx(&self) -> Self::CTX {}

    async fn upstream_peer(
        &self,
        _session: &mut Session,
        _ctx: &mut Self::CTX,
    ) -> PingoraResult<Box<HttpPeer>> {
        let address = self
            .nacos
            .discover(&self.service_name)
            .await
            .map_err(|error| {
                Error::explain(
                    ErrorType::HTTPStatus(503),
                    format!("room-service discovery failed: {error:#}"),
                )
            })?;
        let address = tokio::net::lookup_host(&address)
            .await
            .map_err(|error| {
                Error::explain(
                    ErrorType::HTTPStatus(503),
                    format!("resolve room-service {address}: {error}"),
                )
            })?
            .next()
            .ok_or_else(|| {
                Error::explain(
                    ErrorType::HTTPStatus(503),
                    "room-service address resolved to no IPs",
                )
            })?;
        Ok(Box::new(HttpPeer::new(address, false, String::new())))
    }

    async fn upstream_request_filter(
        &self,
        session: &mut Session,
        upstream_request: &mut RequestHeader,
        _ctx: &mut Self::CTX,
    ) -> PingoraResult<()> {
        if let Some(address) = session.client_addr().and_then(|address| address.as_inet()) {
            let client_ip = address.ip().to_string();
            let forwarded_for = upstream_request
                .headers
                .get("x-forwarded-for")
                .and_then(|value| value.to_str().ok())
                .filter(|value| !value.trim().is_empty())
                .map(|value| format!("{value}, {client_ip}"))
                .unwrap_or(client_ip);
            upstream_request.insert_header("X-Forwarded-For", forwarded_for)?;
        }
        Ok(())
    }

    fn fail_to_connect(
        &self,
        _session: &mut Session,
        _peer: &HttpPeer,
        _ctx: &mut Self::CTX,
        error: Box<Error>,
    ) -> Box<Error> {
        self.nacos.invalidate();
        error
    }
}

fn main() -> Result<()> {
    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or("info")).init();
    let nacos = NacosClient::new(&std::env::var("NACOS_SERVER_ADDR").unwrap_or_default())?;
    let service_name =
        std::env::var("ROOM_SERVICE_NAME").unwrap_or_else(|_| "room-service".to_owned());
    let port = std::env::var("PORT").unwrap_or_else(|_| "8080".to_owned());

    let mut server = Server::new(None)?;
    server.bootstrap();
    let mut proxy = http_proxy_service(
        &server.configuration,
        Gateway {
            nacos,
            service_name,
        },
    );
    proxy.add_tcp(&format!("0.0.0.0:{port}"));
    log::info!("gateway HTTP listening on 0.0.0.0:{port}");

    let cert_path = std::env::var("TLS_CERT_FILE")
        .ok()
        .filter(|value| !value.trim().is_empty());
    let key_path = std::env::var("TLS_KEY_FILE")
        .ok()
        .filter(|value| !value.trim().is_empty());
    if cert_path.is_some() != key_path.is_some() {
        bail!("TLS_CERT_FILE and TLS_KEY_FILE must be configured together");
    }
    if let (Some(cert_path), Some(key_path)) = (cert_path, key_path) {
        let certificate_service = background_service(
            "TLS certificate reload",
            CertificateResolver::new(cert_path, key_path)?,
        );
        let mut tls_settings = TlsSettings::intermediate("", "")?;
        tls_settings.set_cert_resolver(certificate_service.task());
        tls_settings.enable_h2();
        let tls_port = std::env::var("TLS_PORT").unwrap_or_else(|_| "443".to_owned());
        proxy.add_tls_with_settings(&format!("0.0.0.0:{tls_port}"), None, tls_settings);
        server.add_service(certificate_service);
        log::info!("gateway HTTPS listening on 0.0.0.0:{tls_port}");
    }

    server.add_service(proxy);
    server.run_forever();
}
