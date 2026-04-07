//! nfqws process management
//!
//! Spawns, monitors, and gracefully shuts down the nfqws binary.

use crate::config::DaemonConfig;
use crate::error::{Result, UnboundError};
use nix::sys::signal::{kill, Signal};
use nix::unistd::Pid;
use std::fs;
use std::io::Read;
use std::path::Path;
use std::process::Command;
use tracing::{debug, info, warn};

const PID_FILE: &str = "/run/unbound-cli.pid";

/// Start nfqws detached (daemon mode)
pub fn start_nfqws_detached(config: &DaemonConfig) -> Result<u32> {
    if !Path::new(&config.nfqws_path).exists() {
        return Err(UnboundError::NfqwsProcess(format!(
            "nfqws binary not found at '{}'",
            config.nfqws_path
        )));
    }

    let mut args = vec![
        "--qnum".to_string(),
        config.queue_num.to_string(),
        "--dpi-desync".to_string(),
        "fake".to_string(),
        "--dpi-desync-protocol".to_string(),
        "tls".to_string(),
        "--dpi-desync-at".to_string(),
        "sni".to_string(),
        "--dpi-desync-split-pos".to_string(),
        "1".to_string(),
    ];

    if let Some(ref extra) = config.nfqws_args {
        match shell_words::split(extra) {
            Ok(parsed) => args.extend(parsed),
            Err(_) => args.push(extra.clone()),
        }
    }

    debug!("Starting nfqws detached: {} {:?}", config.nfqws_path, args);

    let output = Command::new("nohup")
        .arg(&config.nfqws_path)
        .args(&args)
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null())
        .spawn()
        .map_err(|e| UnboundError::NfqwsProcess(format!("Failed to start nfqws: {}", e)))?;

    let pid = output.id();
    info!("nfqws started (detached) with PID {}", pid);

    write_pid_file(pid)?;
    Ok(pid)
}

/// Stop nfqws process by reading PID file
pub fn stop_nfqws() -> Result<()> {
    let pid = read_pid_file()?;

    info!("Stopping nfqws (PID {})", pid);

    match kill(Pid::from_raw(pid as i32), Signal::SIGTERM) {
        Ok(()) => {
            debug!("Sent SIGTERM to nfqws");
        }
        Err(nix::errno::Errno::ESRCH) => {
            warn!("nfqws process {} already exited", pid);
            cleanup_pid_file().ok();
            return Ok(());
        }
        Err(e) => {
            return Err(UnboundError::Permission(format!(
                "Failed to send SIGTERM: {}",
                e
            )));
        }
    }

    // Wait briefly, then SIGKILL if still alive
    std::thread::sleep(std::time::Duration::from_millis(500));

    if is_process_alive(pid) {
        warn!("nfqws did not exit after SIGTERM, sending SIGKILL");
        if let Err(e) = kill(Pid::from_raw(pid as i32), Signal::SIGKILL) {
            warn!("Failed to send SIGKILL: {}", e);
        }
    }

    cleanup_pid_file()?;
    info!("nfqws stopped");
    Ok(())
}

/// Check if nfqws is running
pub fn nfqws_running() -> bool {
    read_pid_file()
        .map(|pid| is_process_alive(pid))
        .unwrap_or(false)
}

/// Get the current PID if running
pub fn nfqws_pid() -> Option<u32> {
    read_pid_file().ok().filter(|pid| is_process_alive(*pid))
}

// ─── PID file helpers ────────────────────────────────────────────────

fn write_pid_file(pid: u32) -> Result<()> {
    fs::write(PID_FILE, pid.to_string()).map_err(UnboundError::Io)?;
    debug!("Wrote PID {} to {}", pid, PID_FILE);
    Ok(())
}

fn read_pid_file() -> Result<u32> {
    let mut file = fs::File::open(PID_FILE).map_err(|_| UnboundError::NotRunning)?;
    let mut contents = String::new();
    file.read_to_string(&mut contents)
        .map_err(UnboundError::Io)?;
    contents
        .trim()
        .parse::<u32>()
        .map_err(|e| UnboundError::NfqwsProcess(format!("Invalid PID file: {}", e)))
}

fn cleanup_pid_file() -> Result<()> {
    if Path::new(PID_FILE).exists() {
        fs::remove_file(PID_FILE).map_err(UnboundError::Io)?;
    }
    Ok(())
}

fn is_process_alive(pid: u32) -> bool {
    kill(Pid::from_raw(pid as i32), Signal::SIGCONT).is_ok()
}
