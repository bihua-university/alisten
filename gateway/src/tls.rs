use std::{
    sync::{Arc, RwLock},
    time::{Duration, SystemTime},
};

use anyhow::{anyhow, Context, Result};
use async_trait::async_trait;
use pingora::{
    server::ShutdownWatch,
    services::background::BackgroundService,
    tls::{
        hash_certificate, install_default_crypto_provider, load_certs_and_key_files,
        sign::CertifiedKey, ClientHello, CryptoProvider, ResolvesServerCert,
    },
};

const RELOAD_INTERVAL: Duration = Duration::from_secs(30);

#[derive(Debug)]
pub struct CertificateResolver {
    cert_path: String,
    key_path: String,
    certified_key: RwLock<Arc<CertifiedKey>>,
    version: RwLock<CertificateVersion>,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
struct CertificateVersion {
    cert_modified: SystemTime,
    key_modified: SystemTime,
}

impl CertificateResolver {
    pub fn new(cert_path: String, key_path: String) -> Result<Self> {
        let certified_key = load_certified_key(&cert_path, &key_path)?;
        let version = certificate_version(&cert_path, &key_path)?;
        Ok(Self {
            cert_path,
            key_path,
            certified_key: RwLock::new(certified_key),
            version: RwLock::new(version),
        })
    }

    fn reload_if_changed(&self) -> Result<bool> {
        let version = certificate_version(&self.cert_path, &self.key_path)?;
        if *self
            .version
            .read()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
            == version
        {
            return Ok(false);
        }
        let certified_key = load_certified_key(&self.cert_path, &self.key_path)?;
        *self
            .certified_key
            .write()
            .unwrap_or_else(|poisoned| poisoned.into_inner()) = Arc::clone(&certified_key);
        *self
            .version
            .write()
            .unwrap_or_else(|poisoned| poisoned.into_inner()) = version;
        log::info!(
            "reloaded TLS certificate {:x?}",
            hash_certificate(certified_key.end_entity_cert()?)
        );
        Ok(true)
    }
}

impl ResolvesServerCert for CertificateResolver {
    fn resolve(&self, _client_hello: ClientHello<'_>) -> Option<Arc<CertifiedKey>> {
        Some(
            self.certified_key
                .read()
                .unwrap_or_else(|poisoned| poisoned.into_inner())
                .clone(),
        )
    }
}

#[async_trait]
impl BackgroundService for CertificateResolver {
    async fn start(&self, mut shutdown: ShutdownWatch) {
        let mut interval = tokio::time::interval(RELOAD_INTERVAL);
        loop {
            tokio::select! {
                _ = shutdown.changed() => return,
                _ = interval.tick() => {
                    if let Err(error) = self.reload_if_changed() {
                        log::warn!("reload TLS certificate failed; retaining previous certificate: {error:#}");
                    }
                }
            }
        }
    }
}

fn load_certified_key(cert_path: &str, key_path: &str) -> Result<Arc<CertifiedKey>> {
    install_default_crypto_provider();
    let (certs, key) = load_certs_and_key_files(cert_path, key_path)?
        .ok_or_else(|| anyhow!("certificate or private key is missing"))?;
    let provider = CryptoProvider::get_default()
        .ok_or_else(|| anyhow!("TLS crypto provider is unavailable"))?;
    let certified_key = CertifiedKey::from_der(certs, key, provider)
        .context("parse TLS certificate and private key")?;
    Ok(Arc::new(certified_key))
}

fn certificate_version(cert_path: &str, key_path: &str) -> Result<CertificateVersion> {
    Ok(CertificateVersion {
        cert_modified: std::fs::metadata(cert_path)?.modified()?,
        key_modified: std::fs::metadata(key_path)?.modified()?,
    })
}

#[cfg(test)]
mod tests {
    use std::{
        fs,
        path::{Path, PathBuf},
        time::{SystemTime, UNIX_EPOCH},
    };

    use super::*;

    #[test]
    fn reloads_changed_certificate() {
        let directory = test_directory();
        fs::create_dir_all(&directory).unwrap();
        write_certificate(&directory, "first.example");
        let resolver = CertificateResolver::new(
            directory
                .join("fullchain.pem")
                .to_string_lossy()
                .into_owned(),
            directory.join("privkey.pem").to_string_lossy().into_owned(),
        )
        .unwrap();
        let first = resolver
            .certified_key
            .read()
            .unwrap()
            .end_entity_cert()
            .unwrap()
            .as_ref()
            .to_vec();

        std::thread::sleep(Duration::from_millis(20));
        write_certificate(&directory, "second.example");
        assert!(resolver.reload_if_changed().unwrap());
        let second = resolver
            .certified_key
            .read()
            .unwrap()
            .end_entity_cert()
            .unwrap()
            .as_ref()
            .to_vec();
        assert_ne!(first, second);
        fs::remove_dir_all(directory).unwrap();
    }

    fn write_certificate(directory: &Path, hostname: &str) {
        let certificate = rcgen::generate_simple_self_signed([hostname.to_owned()]).unwrap();
        fs::write(directory.join("fullchain.pem"), certificate.cert.pem()).unwrap();
        fs::write(
            directory.join("privkey.pem"),
            certificate.signing_key.serialize_pem(),
        )
        .unwrap();
    }

    fn test_directory() -> PathBuf {
        let nonce = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("target")
            .join(format!("tls-test-{}-{nonce}", std::process::id()))
    }
}
