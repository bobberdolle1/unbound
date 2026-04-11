//! nftables rule management
//!
//! Creates and removes nftables rules that redirect HTTP/HTTPS/QUIC traffic
//! to the NFQUEUE that nfqws listens on. Uses the `nft` CLI for reliable
//! idempotent operations.
//!
//! Table structure:
//!   table inet unbound {
//!       chain post {
//!           type filter hook postrouting priority mangle;
//!           oifname "eth0" meta mark and 0x40000000 == 0 tcp dport {80,443} ct original packets 1-6 queue num 200 bypass
//!           oifname "eth0" meta mark and 0x40000000 == 0 udp dport {443} ct original packets 1-6 queue num 200 bypass
//!       }
//!       chain pre {
//!           type filter hook prerouting priority filter;
//!           iifname "eth0" tcp sport {80,443} ct reply packets 1-3 queue num 200 bypass
//!       }
//!   }

use crate::config::DaemonConfig;
use crate::error::{Result, UnboundError};
use std::process::Command;
use tracing::{debug, info, warn};

const TABLE_NAME: &str = "unbound";
const CHAIN_POST: &str = "post";
const CHAIN_PRE: &str = "pre";
const ZAPRET_MARK: u32 = 0x4000_0000;

/// Detect the default WAN interface
pub fn detect_wan_interface() -> Result<String> {
    let output = Command::new("sh")
        .args([
            "-c",
            "ip route get 1.1.1.1 2>/dev/null | grep -oP 'dev \\K\\S+' | head -n1",
        ])
        .output()
        .map_err(UnboundError::Io)?;

    if output.status.success() {
        let iface = String::from_utf8_lossy(&output.stdout);
        let iface = iface.trim();
        if !iface.is_empty() {
            debug!("Detected WAN interface: {}", iface);
            return Ok(iface.to_string());
        }
    }

    for fallback in &["eth0", "enp0s3", "wlan0", "wlp2s0"] {
        let test = Command::new("ip")
            .args(["link", "show", fallback])
            .output()
            .map_err(UnboundError::Io)?;
        if test.status.success() {
            debug!("Using fallback WAN interface: {}", fallback);
            return Ok(fallback.to_string());
        }
    }

    Err(UnboundError::Config(
        "Could not detect WAN interface. Specify with --iface".into(),
    ))
}

/// Apply nftables rules for DPI bypass
pub fn apply_rules(config: &DaemonConfig) -> Result<()> {
    let iface = match &config.iface {
        Some(iface) => iface.clone(),
        None => detect_wan_interface()?,
    };

    let tcp_ports = config.tcp_ports_vec();
    let udp_ports = config.udp_ports_vec();
    let queue = config.queue_num;

    info!("Applying nftables rules on interface '{}' to NFQUEUE {}", iface, queue);

    // Remove existing table if present (idempotent)
    remove_rules().ok();

    let mut cmds: Vec<String> = Vec::new();

    // Create table and chains
    cmds.push(format!("add table inet {}", TABLE_NAME));
    cmds.push(format!(
        "add chain inet {} {} {{ type filter hook postrouting priority mangle; policy accept; }}",
        TABLE_NAME, CHAIN_POST
    ));
    cmds.push(format!(
        "add chain inet {} {} {{ type filter hook prerouting priority filter; policy accept; }}",
        TABLE_NAME, CHAIN_PRE
    ));

    // TCP rule: outgoing HTTP/HTTPS
    let tcp_ports_str = tcp_ports.join(",");
    cmds.push(format!(
        "add rule inet {} {} oifname \"{}\" meta mark and 0x{:x} == 0 tcp dport {{ {} }} ct original packets 1-6 queue num {} bypass",
        TABLE_NAME, CHAIN_POST, iface, ZAPRET_MARK, tcp_ports_str, queue
    ));

    // UDP rule: outgoing QUIC
    let udp_ports_str = udp_ports.join(",");
    cmds.push(format!(
        "add rule inet {} {} oifname \"{}\" meta mark and 0x{:x} == 0 udp dport {{ {} }} ct original packets 1-6 queue num {} bypass",
        TABLE_NAME, CHAIN_POST, iface, ZAPRET_MARK, udp_ports_str, queue
    ));

    // Optional: incoming reply tracking
    cmds.push(format!(
        "add rule inet {} {} iifname \"{}\" tcp sport {{ {} }} ct reply packets 1-3 queue num {} bypass",
        TABLE_NAME, CHAIN_PRE, iface, tcp_ports_str, queue
    ));

    // Apply all rules atomically via stdin
    let nft_input = cmds.join("\n");
    debug!("nft commands:\n{}", nft_input);

    let mut child = Command::new("nft")
        .arg("-f")
        .arg("-")
        .stdin(std::process::Stdio::piped())
        .stderr(std::process::Stdio::piped())
        .spawn()
        .map_err(UnboundError::Io)?;

    if let Some(mut stdin) = child.stdin.take() {
        use std::io::Write;
        stdin.write_all(nft_input.as_bytes()).map_err(UnboundError::Io)?;
    }

    let output = child.wait().map_err(UnboundError::Io)?;

    if !output.success() {
        return Err(UnboundError::Nftables(
            "nft command failed".into(),
        ));
    }

    info!("nftables rules applied successfully");
    Ok(())
}

/// Remove all unbound nftables rules
pub fn remove_rules() -> Result<()> {
    debug!("Removing nftables table 'inet {}'", TABLE_NAME);

    let output = Command::new("nft")
        .args(["delete", "table", "inet", TABLE_NAME])
        .output()
        .map_err(UnboundError::Io)?;

    if output.status.success() {
        info!("nftables rules removed");
    } else {
        let stderr = String::from_utf8_lossy(&output.stderr);
        if stderr.contains("No such file") || stderr.contains("does not exist") {
            debug!("nftables table did not exist (no-op)");
        } else {
            warn!("nft delete returned non-zero: {}", stderr.trim());
        }
    }

    Ok(())
}

/// Check if unbound nftables rules are currently active
pub fn rules_active() -> bool {
    let output = match Command::new("nft")
        .args(["list", "table", "inet", TABLE_NAME])
        .output()
    {
        Ok(o) => o,
        Err(_) => return false,
    };

    output.status.success()
}

/// Flush rules without deleting the table
pub fn flush_rules() -> Result<()> {
    let output = Command::new("nft")
        .args(["flush", "table", "inet", TABLE_NAME])
        .output()
        .map_err(UnboundError::Io)?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        if !stderr.contains("does not exist") {
            warn!("nft flush returned non-zero: {}", stderr.trim());
        }
    }

    Ok(())
}
