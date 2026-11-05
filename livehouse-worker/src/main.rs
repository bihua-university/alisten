mod api;
mod model;
mod protocol;
mod room;
mod store;

use std::{sync::Arc, time::Duration};

use anyhow::Result;
use api::AppState;
use hyper_util::{
    rt::{TokioExecutor, TokioIo},
    server::conn::auto::Builder,
    service::TowerToHyperService,
};
use room::RoomManager;
use store::{RedisStore, StateStore};
use tracing_subscriber::EnvFilter;
use uuid::Uuid;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            EnvFilter::try_from_default_env().unwrap_or_else(|_| "livehouse_worker=info".into()),
        )
        .init();

    let redis_url =
        std::env::var("REDIS_URL").unwrap_or_else(|_| "redis://127.0.0.1:6379".to_owned());
    let prefix = std::env::var("LIVEHOUSE_REDIS_PREFIX").unwrap_or_else(|_| "livehouse".to_owned());
    let store = Arc::new(RedisStore::connect(&redis_url, prefix).await?);
    let store_trait: Arc<dyn StateStore> = store.clone();
    let node_id = std::env::var("LIVEHOUSE_NODE_ID").unwrap_or_else(|_| Uuid::new_v4().to_string());
    let max_members = std::env::var("LIVEHOUSE_MAX_MEMBERS")
        .ok()
        .and_then(|value| value.parse().ok())
        .filter(|value| *value > 0)
        .unwrap_or(100);
    let rooms = Arc::new(RoomManager::new(store_trait, node_id.clone(), max_members));
    let internal_token = std::env::var("INTERNAL_API_TOKEN")
        .ok()
        .map(|value| value.trim().to_owned())
        .filter(|value| !value.is_empty());
    let state = AppState {
        rooms: rooms.clone(),
        internal_token: internal_token.map(Arc::from),
        node_id: Arc::from(node_id.clone()),
        store: store.clone(),
        http: reqwest::Client::new(),
    };

    let listen_address =
        std::env::var("LIVEHOUSE_ADDR").unwrap_or_else(|_| "127.0.0.1:8082".to_owned());
    store.heartbeat(&node_id, &listen_address).await?;

    let heartbeat_store = store.clone();
    let heartbeat_node = node_id.clone();
    let heartbeat_address = listen_address.clone();
    let heartbeat = tokio::spawn(async move {
        let mut interval = tokio::time::interval(Duration::from_secs(5));
        loop {
            interval.tick().await;
            if let Err(error) = heartbeat_store
                .heartbeat(&heartbeat_node, &heartbeat_address)
                .await
            {
                tracing::error!(%error, "node heartbeat");
            }
        }
    });

    let lease_rooms = rooms.clone();
    let leases = tokio::spawn(async move {
        let mut interval = tokio::time::interval(Duration::from_millis(3_000));
        loop {
            interval.tick().await;
            lease_rooms.renew_leases().await;
        }
    });
    let playback_rooms = rooms.clone();
    let playback = tokio::spawn(async move {
        let mut interval = tokio::time::interval(Duration::from_millis(250));
        loop {
            interval.tick().await;
            playback_rooms.tick().await;
        }
    });

    let listener = tokio::net::TcpListener::bind(&listen_address).await?;
    tracing::info!(address = %listener.local_addr()?, %node_id, "livehouse-worker listening");
    let shutdown = async {
        let _ = tokio::signal::ctrl_c().await;
    };
    tokio::pin!(shutdown);
    loop {
        tokio::select! {
            _ = &mut shutdown => break,
            accepted = listener.accept() => {
                let (stream, _) = accepted?;
                let state = state.clone();
                tokio::spawn(async move {
                    let service = TowerToHyperService::new(api::router(state));
                    let builder = Builder::new(TokioExecutor::new());
                    let connection = builder
                        .serve_connection_with_upgrades(TokioIo::new(stream), service);
                    if let Err(error) = connection.await {
                        tracing::debug!(%error, "HTTP connection closed");
                    }
                });
            }
        }
    }

    heartbeat.abort();
    leases.abort();
    playback.abort();
    rooms.shutdown().await;
    store.unregister_node(&node_id).await?;
    Ok(())
}
