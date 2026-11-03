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
    canary_namespace: Option<String>,
}

fn selected_namespace<'a>(
    request: &RequestHeader,
    canary_namespace: Option<&'a str>,
) -> Option<&'a str> {
    let is_canary = request.uri.query().is_some_and(|query| {
        url::form_urlencoded::parse(query.as_bytes())
            .any(|(key, value)| key == "env" && value == "canary")
    }) || request
        .headers
        .get("X-Alisten-Env")
        .is_some_and(|value| value == "canary");
    canary_namespace.filter(|value| !value.trim().is_empty() && is_canary)
}

#[async_trait]
impl ProxyHttp for Gateway {
    type CTX = ();

    fn new_ctx(&self) -> Self::CTX {}

    async fn upstream_peer(
        &self,
        session: &mut Session,
        _ctx: &mut Self::CTX,
    ) -> PingoraResult<Box<HttpPeer>> {
        let namespace = selected_namespace(session.req_header(), self.canary_namespace.as_deref());
        log::info!(
            "upstream discovery service={} namespace={}",
            self.service_name,
            match self.nacos.namespace(namespace) {
                "" => "public",
                value => value,
            }
        );
        let address = self
            .nacos
            .discover(&self.service_name, namespace)
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
    let canary_namespace = std::env::var("CANARY_NACOS_NAMESPACE")
        .ok()
        .map(|value| value.trim().to_owned())
        .filter(|value| !value.is_empty());
    let port = std::env::var("PORT").unwrap_or_else(|_| "8080".to_owned());

    let mut server = Server::new(None)?;
    server.bootstrap();
    let mut proxy = http_proxy_service(
        &server.configuration,
        Gateway {
            nacos,
            service_name,
            canary_namespace,
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn routes_query_and_header_only_when_canary_enabled() {
        let mut request = RequestHeader::build(
            "GET",
            b"/server?houseId=test&housePwd=a%26b&env=canary",
            None,
        )
        .unwrap();
        assert_eq!(
            selected_namespace(&request, Some("preview")),
            Some("preview")
        );
        assert_eq!(selected_namespace(&request, None), None);
        assert_eq!(selected_namespace(&request, Some("")), None);
        request = RequestHeader::build("GET", b"/house/search?env=prod", None).unwrap();
        assert_eq!(selected_namespace(&request, Some("canary")), None);
        request.insert_header("X-Alisten-Env", "canary").unwrap();
        assert_eq!(selected_namespace(&request, Some("canary")), Some("canary"));
    }
}
