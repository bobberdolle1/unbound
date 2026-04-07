# Unbound-WRT — OpenWrt DPI Bypass Package

Router-level transparent DPI/censorship bypass for the entire LAN. Zero client-side configuration required.

## Directory Structure

```
openwrt/
├── README.md                          # This file
├── unbound-wrt/                       # Core nfqws package
│   ├── Makefile                       # OpenWrt package Makefile
│   └── files/
│       ├── etc/config/unbound         # UCI default configuration
│       ├── etc/init.d/unbound         # procd init script
│       └── etc/nftables.d/90-unbound-wrt.nft  # fw4/nftables rules
│
└── luci-app-unbound/                  # LuCI web interface
    ├── Makefile                       # LuCI package Makefile
    └── luasrc/
        ├── controller/unbound.lua     # Menu registration + status API
        └── model/cbi/unbound/unbound.lua  # CBI configuration model
```

## Architecture

```
LAN Clients (zero config)
    │ br-lan (TCP 80/443)
    ▼
fw4 / nftables (90-unbound-wrt.nft)
  - Intercepts forward traffic from br-lan
  - Excludes: RFC1918, broadcast, router itself
  - Sends matching packets to NFQUEUE 200
    ▼
nfqws daemon (procd-managed)
  - Receives packets via NFQUEUE 200
  - Applies strategy: disorder, split-tls, fake, etc.
  - Re-injects mangled packets to kernel stack
    ▼
WAN / Uplink
```

## Building

### Prerequisites

- OpenWrt SDK matching your target version (22.03 or 23.05)
- `libnetfilter-queue` and `libnetfilter-conntrack` in SDK feeds

### Steps

1. **Clone the OpenWrt SDK:**
   ```bash
   wget https://downloads.openwrt.org/releases/23.05.0/targets/<arch>/<target>/openwrt-sdk-23.05.0-<arch>-<target>.Linux-x86_64.tar.xz
   tar xf openwrt-sdk-*.tar.xz
   cd openwrt-sdk-*
   ```

2. **Copy packages into the SDK:**
   ```bash
   cp -r /path/to/unbound-wrt package/
   cp -r /path/to/luci-app-unbound package/
   ```

3. **Update feeds:**
   ```bash
   ./scripts/feeds update -a
   ./scripts/feeds install -a
   ```

4. **Select packages in menuconfig:**
   ```bash
   make menuconfig
   ```
   - `Network > Web Servers/Proxies > nfqws-unbound` → set to `M`
   - `LuCI > 3. Applications > luci-app-unbound` → set to `M`

5. **Compile:**
   ```bash
   make package/nfqws-unbound/compile V=s
   make package/luci-app-unbound/compile V=s
   ```

6. **Output `.ipk` files:**
   ```
   bin/packages/<arch>/base/nfqws-unbound_*.ipk
   bin/packages/<arch>/luci/luci-app-unbound_*.ipk
   ```

## Installation

```bash
# Transfer to router
scp bin/packages/*/base/nfqws-unbound_*.ipk root@192.168.1.1:/tmp/
scp bin/packages/*/luci/luci-app-unbound_*.ipk root@192.168.1.1:/tmp/

# Install on router
ssh root@192.168.1.1
opkg install /tmp/nfqws-unbound_*.ipk
opkg install /tmp/luci-app-unbound_*.ipk

# Enable and start
/etc/init.d/unbound enable
/etc/init.d/unbound start
```

## Configuration

### Via LuCI Web Interface

Navigate to **Services > Unbound-WRT** in LuCI:

| Setting | Description |
|---------|-------------|
| **Enable** | Master toggle for the DPI bypass engine |
| **Bypass Strategy** | Packet mangling strategy (see below) |
| **Exclude Domains** | Domains that bypass nfqws (one per line) |
| **Exclude IPs** | IP/CIDR ranges that bypass nfqws (one per line) |

### Via CLI (UCI)

```bash
uci set unbound.@general[0].enabled='1'
uci set unbound.@general[0].strategy='multidisorder'
uci set unbound.@general[0].exclude_ips='192.168.1.100 10.0.0.0/8'
uci commit unbound
/etc/init.d/unbound restart
```

## Bypass Strategies

| Strategy | Description | Best For |
|----------|-------------|----------|
| **Multidisorder** | Disorders packet segments | General purpose |
| **Split TLS** | Splits TLS ClientHello | TLS-based SNI blocking |
| **Fake Ping** | Injects fake low-TTL packets | Aggressive DPI |
| **Disorder + Fake** | Combines disorder + fake injection | Maximum evasion |

## Installed Files

| Path | Purpose |
|------|---------|
| `/usr/bin/nfqws` | NFQUEUE userspace daemon (cross-compiled C binary) |
| `/etc/init.d/unbound` | procd service manager script |
| `/etc/config/unbound` | UCI configuration file |
| `/etc/nftables.d/90-unbound-wrt.nft` | nftables interception rules |

## Troubleshooting

```bash
# Check service status
/etc/init.d/unbound status
logread | grep nfqws

# Verify nftables rules
nft list chain inet fw4 unbound_wrt_forward
nft list chain inet fw4 unbound_wrt_lan_check

# Check NFQUEUE is receiving packets
nft list ruleset | grep queue

# Test connectivity from LAN client
tcpdump -i br-lan tcp port 443
```

## License

GPL-3.0-only
