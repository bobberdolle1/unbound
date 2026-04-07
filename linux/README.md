# Unbound Linux — DPI/Censorship Bypass Daemon & Decky Plugin

High-performance DPI/censorship bypass tool for Linux, built in Rust. Wraps the
C-based `nfqws` binary from the [zapret](https://github.com/bol-van/zapret)
project with automated nftables rule management.

## Architecture

```
User Interface
  CLI (sudo)          Decky Plugin (Game Mode)
  unbound-cli         Preact UI + Python backend
          \               /
           v             v
        unbound-cli daemon (Rust, runs as root)
           /            \
      nftables        nfqws (zapret)
      rules           process
     (NFQUEUE)
```

## Components

| File | Description |
|------|-------------|
| `src/main.rs` | CLI entry with `start`, `stop`, `status` subcommands + `nfqws` auto-detection |
| `src/nftables_mgr.rs` | Dynamic nftables rule apply/remove/flush using zapret-compatible syntax |
| `src/nfqws.rs` | Process manager: spawn/stop/monitor `nfqws` via PID file, SIGTERM to SIGKILL graceful shutdown |
| `src/daemon.rs` | Lifecycle orchestrator: applies rules, starts nfqws, waits for SIGINT/SIGTERM, flushes everything |
| `src/config.rs` | Daemon configuration struct with port parsing |
| `src/error.rs` | Typed error enum |

## Build

```bash
cargo build --release
# Binary: ../target/release/unbound-cli
```

## Usage (requires root)

```bash
# Start with defaults (queue 200, auto-detect interface)
sudo unbound-cli start

# Start with specific interface and queue
sudo unbound-cli start --iface eth0 --queue 200

# Check status
sudo unbound-cli status

# Stop (flushes nftables rules automatically)
sudo unbound-cli stop
```

## nftables Rules

When active, the daemon creates:

```
table inet unbound {
    chain post {
        type filter hook postrouting priority mangle;
        oifname "eth0" meta mark and 0x40000000 == 0 tcp dport {80,443} ct original packets 1-6 queue num 200 bypass
        oifname "eth0" meta mark and 0x40000000 == 0 udp dport {443} ct original packets 1-6 queue num 200 bypass
    }
    chain pre {
        type filter hook prerouting priority filter;
        iifname "eth0" tcp sport {80,443} ct reply packets 1-3 queue num 200 bypass
    }
}
```

Rules are **automatically removed** on daemon shutdown (SIGINT/SIGTERM/crash),
ensuring the user is never left without internet.

## systemd Service

```bash
sudo cp ../packaging/unbound.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now unbound.service
```

## Dependencies

- **Runtime:** `nftables`, `libnetfilter_queue`, `nfqws` (from zapret)
- **Build:** `cargo`, `rust`

## Packaging

See `../packaging/` for:
- `PKGBUILD` — Arch Linux AUR
- `build-deb.sh` — Debian/Ubuntu
- `build-rpm.sh` — Fedora/RHEL
- `unbound.service` — systemd service file

## License

MIT
