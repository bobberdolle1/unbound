//! Unbound CLI — Linux DPI/censorship bypass daemon
//!
//! Wraps the C-based `nfqws` binary from the zapret project, managing
//! nftables rules to route HTTP/HTTPS/QUIC traffic through NFQUEUE.

mod nftables_mgr;
mod nfqws;
mod config;
mod error;
mod daemon;

use clap::{Parser, Subcommand};
use tracing_subscriber::EnvFilter;

#[derive(Parser)]
#[command(name = "unbound-cli")]
#[command(about = "Linux DPI/censorship bypass daemon", long_about = None)]
struct Cli {
    /// Enable verbose debug output
    #[arg(short, long, global = true)]
    verbose: bool,

    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Start the DPI bypass daemon
    Start {
        /// NFQUEUE number (default: 200)
        #[arg(short, long, default_value_t = 200)]
        queue: u32,

        /// Path to nfqws binary (auto-detect if not specified)
        #[arg(short, long)]
        nfqws_path: Option<String>,

        /// Ports to intercept (default: 80,443 for TCP; 443 for UDP)
        #[arg(short, long, default_value = "80,443")]
        tcp_ports: String,

        /// UDP ports to intercept (default: 443 for QUIC)
        #[arg(long, default_value = "443")]
        udp_ports: String,

        /// WAN interface name (auto-detect if not specified)
        #[arg(short, long)]
        iface: Option<String>,

        /// Custom strategies for nfqws (e.g., "--wssize 1:6 --dpi-heuristic=sni")
        #[arg(long)]
        nfqws_args: Option<String>,
    },
    /// Stop the DPI bypass daemon and flush nftables rules
    Stop,
    /// Show the current status of the daemon
    Status,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();

    // Initialize tracing
    let filter = if cli.verbose {
        EnvFilter::new("debug")
    } else {
        EnvFilter::new("info")
    };
    tracing_subscriber::fmt()
        .with_target(false)
        .with_env_filter(filter)
        .init();

    match cli.command {
        Commands::Start {
            queue,
            nfqws_path,
            tcp_ports,
            udp_ports,
            iface,
            nfqws_args,
        } => {
            let config = config::DaemonConfig {
                queue_num: queue,
                nfqws_path: nfqws_path.unwrap_or_else(|| detect_nfqws()),
                tcp_ports,
                udp_ports,
                iface,
                nfqws_args,
            };
            daemon::run(config).await?;
        }
        Commands::Stop => {
            daemon::stop().await?;
        }
        Commands::Status => {
            daemon::status().await?;
        }
    }

    Ok(())
}

/// Attempt to locate nfqws binary in common paths
fn detect_nfqws() -> String {
    use std::path::Path;

    const CANDIDATES: &[&str] = &[
        "/usr/local/bin/nfqws",
        "/usr/bin/nfqws",
        "/usr/sbin/nfqws",
        "/home/deck/homebrew/sbin/nfqws",
        "/home/deck/homebrew/bin/nfqws",
        "/home/deck/homebrew/plugins/unbound/bin/nfqws",
    ];

    for candidate in CANDIDATES {
        if Path::new(candidate).exists() {
            return candidate.to_string();
        }
    }

    if let Ok(path) = which::which("nfqws") {
        return path.to_string_lossy().to_string();
    }

    "/usr/local/bin/nfqws".to_string()
}
