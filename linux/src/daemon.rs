//! Daemon lifecycle management
//!
//! Orchestrates nftables rule application, nfqws process management,
//! and graceful shutdown on signals.

use crate::config::DaemonConfig;
use crate::error::Result;
use crate::nftables_mgr;
use crate::nfqws;
use tokio::signal::unix::{signal, SignalKind};
use tracing::{error, info};

/// Run the daemon: apply rules, start nfqws, wait for shutdown signal
pub async fn run(config: DaemonConfig) -> Result<()> {
    // Check if already running
    if nfqws::nfqws_running() {
        if let Some(pid) = nfqws::nfqws_pid() {
            return Err(crate::error::UnboundError::AlreadyRunning(pid.to_string()));
        }
    }

    // Check root privileges
    if !is_root() {
        return Err(crate::error::UnboundError::Permission(
            "unbound-cli requires root privileges (sudo)".into(),
        ));
    }

    info!("Starting unbound daemon...");

    // Apply nftables rules
    nftables_mgr::apply_rules(&config)?;

    // Start nfqws
    nfqws::start_nfqws_detached(&config)?;

    info!("===========================================");
    info!("  Unbound DPI bypass is ACTIVE");
    info!("===========================================");
    info!("Press Ctrl+C to stop...");

    // Wait for SIGINT or SIGTERM
    wait_for_shutdown().await;

    // Graceful shutdown
    shutdown().await
}

/// Stop the daemon and flush rules
pub async fn stop() -> Result<()> {
    info!("Stopping unbound daemon...");
    shutdown().await
}

/// Show current daemon status
pub async fn status() -> Result<()> {
    let running = nfqws::nfqws_running();
    let rules = nftables_mgr::rules_active();

    println!("+------------------------------------------+");
    println!("|         Unbound Status Report            |");
    println!("+------------------------------------------+");

    if running {
        let pid = nfqws::nfqws_pid().unwrap_or(0);
        println!("| nfqws:   {:<24} |", format!("RUNNING (PID {})", pid));
    } else {
        println!("| nfqws:   {:<24} |", "STOPPED");
    }

    if rules {
        println!("| nftables: {:<23} |", "ACTIVE");
    } else {
        println!("| nftables: {:<23} |", "INACTIVE");
    }

    if running && rules {
        println!("| Status:  {:<24} |", "OK - BYPASS ACTIVE");
    } else if !running && !rules {
        println!("| Status:  {:<24} |", "Not running");
    } else {
        println!("| Status:  {:<24} |", "INCONSISTENT STATE");
    }

    println!("+------------------------------------------+");
    Ok(())
}

/// Graceful shutdown: stop nfqws, remove nftables rules
async fn shutdown() -> Result<()> {
    info!("Shutting down...");

    if let Err(e) = nfqws::stop_nfqws() {
        error!("Error stopping nfqws: {}", e);
    }

    if let Err(e) = nftables_mgr::remove_rules() {
        error!("Error removing nftables rules: {}", e);
    }

    info!("Unbound daemon stopped. Internet connectivity restored.");
    Ok(())
}

/// Wait for SIGINT or SIGTERM
async fn wait_for_shutdown() {
    let mut sigint = signal(SignalKind::interrupt()).expect("Failed to register SIGINT handler");
    let mut sigterm = signal(SignalKind::terminate()).expect("Failed to register SIGTERM handler");

    tokio::select! {
        _ = sigint.recv() => {
            info!("Received SIGINT");
        }
        _ = sigterm.recv() => {
            info!("Received SIGTERM");
        }
    }
}

/// Check if running as root
fn is_root() -> bool {
    unsafe { libc::getuid() == 0 }
}
