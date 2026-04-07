//! Daemon configuration

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DaemonConfig {
    /// NFQUEUE number (default: 200)
    pub queue_num: u32,
    /// Path to the nfqws binary
    pub nfqws_path: String,
    /// TCP ports to intercept (comma-separated)
    pub tcp_ports: String,
    /// UDP ports to intercept (comma-separated)
    pub udp_ports: String,
    /// WAN interface (optional, auto-detect)
    pub iface: Option<String>,
    /// Additional nfqws arguments
    pub nfqws_args: Option<String>,
}

impl DaemonConfig {
    pub fn tcp_ports_vec(&self) -> Vec<String> {
        self.tcp_ports
            .split(',')
            .map(|s| s.trim().to_string())
            .collect()
    }

    pub fn udp_ports_vec(&self) -> Vec<String> {
        self.udp_ports
            .split(',')
            .map(|s| s.trim().to_string())
            .collect()
    }
}
