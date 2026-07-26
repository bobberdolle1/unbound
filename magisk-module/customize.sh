#!/system/bin/sh
# ============================================================================
# Unbound Core — Magisk/KernelSU Module Installation Script
# ============================================================================
# This script runs during module installation in Magisk/KernelSU.
# It sets up the directory structure, extracts binaries, and configures
# the module for system-wide DPI bypass.
# ============================================================================

MODDIR="${0%/*}"
MODID="unbound-core"

# Print banner
ui_print " "
ui_print "╔══════════════════════════════════════════╗"
ui_print "║          Unbound Core Installer          ║"
ui_print "║   System-wide DPI/Censorship Bypass      ║"
ui_print "╚══════════════════════════════════════════╝"
ui_print " "

# Detect architecture
ARCH=$(getprop ro.product.cpu.abi)
case "$ARCH" in
    arm64*)
        ARCH_DIR="arm64"
        ui_print "→ Detected architecture: arm64-v8a"
        ;;
    armeabi*)
        ARCH_DIR="arm"
        ui_print "→ Detected architecture: armeabi-v7a"
        ;;
    x86_64*)
        ARCH_DIR="x86_64"
        ui_print "→ Detected architecture: x86_64"
        ;;
    x86*)
        ARCH_DIR="x86"
        ui_print "→ Detected architecture: x86"
        ;;
    *)
        ui_print "! Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Create directory structure
ui_print "→ Creating directory structure..."
mkdir -p "$MODDIR/bin"
mkdir -p "$MODDIR/etc"
mkdir -p "$MODDIR/log"

# Extract nfqws binary (pre-compiled for target architecture)
#
# Abort if it is missing. binaries/ ships with nothing but a .gitkeep
# explaining that the binaries "should be placed" there, so a module packaged
# straight from this repository contains no engine at all. Without this check
# the cp simply printed an error, installation reported success, and the
# failure only surfaced later as service.sh trying to exec a file that was
# never there.
ui_print "→ Extracting nfqws binary..."
if [ ! -f "$MODDIR/binaries/$ARCH_DIR/nfqws" ]; then
    ui_print "!"
    ui_print "! No nfqws binary for $ARCH_DIR in this package."
    ui_print "! Build it from https://github.com/bol-van/zapret with the"
    ui_print "! Android NDK and place it at binaries/$ARCH_DIR/nfqws before"
    ui_print "! packaging the module. See magisk-module/README.md."
    ui_print "!"
    abort "! Installation aborted: the module would not work."
fi
cp "$MODDIR/binaries/$ARCH_DIR/nfqws" "$MODDIR/bin/nfqws" || \
    abort "! Failed to copy the nfqws binary."
chmod 755 "$MODDIR/bin/nfqws"

# Extract iptables/nftables scripts
ui_print "→ Installing iptables scripts..."
cp "$MODDIR/scripts/iptables_setup.sh" "$MODDIR/bin/iptables_setup.sh"
chmod 755 "$MODDIR/bin/iptables_setup.sh"

cp "$MODDIR/scripts/iptables_cleanup.sh" "$MODDIR/bin/iptables_cleanup.sh"
chmod 755 "$MODDIR/bin/iptables_cleanup.sh"

# Extract configuration
ui_print "→ Installing configuration..."
if [ ! -f "$MODDIR/etc/unbound.conf" ]; then
    cp "$MODDIR/config/unbound.conf.default" "$MODDIR/etc/unbound.conf"
    ui_print "  → Created default config: unbound.conf"
fi

# Set permissions
ui_print "→ Setting permissions..."
chmod 644 "$MODDIR/module.prop"
chmod 755 "$MODDIR/service.sh"
chmod 755 "$MODDIR/uninstall.sh"

# Create log file
touch "$MODDIR/log/nfqws.log"
chmod 666 "$MODDIR/log/nfqws.log"

ui_print " "
ui_print "✓ Installation complete!"
ui_print " "
ui_print "→ Configuration file: $MODDIR/etc/unbound.conf"
ui_print "→ Log file: $MODDIR/log/nfqws.log"
ui_print " "
ui_print "→ The module will start automatically on next reboot."
ui_print "→ To start manually: /data/adb/modules/unbound-core/service.sh start"
ui_print "→ To stop: /data/adb/modules/unbound-core/service.sh stop"
ui_print " "
