//! Error types for unbound-cli

use thiserror::Error;

#[derive(Error, Debug)]
pub enum UnboundError {
    #[error("nftables error: {0}")]
    Nftables(String),

    #[error("nfqws process error: {0}")]
    NfqwsProcess(String),

    #[error("daemon already running (PID file: {0})")]
    AlreadyRunning(String),

    #[error("daemon not running")]
    NotRunning,

    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),

    #[error("configuration error: {0}")]
    Config(String),

    #[error("permission denied: {0}")]
    Permission(String),
}

pub type Result<T> = std::result::Result<T, UnboundError>;
