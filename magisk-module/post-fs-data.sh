#!/system/bin/sh
# ============================================================================
# Unbound Core — post-fs-data.sh
# ============================================================================
# This script runs early in the boot process (post-fs-data mode).
# Use it to set up early-stage configurations that must be ready
# before the service.sh script runs.
# ============================================================================

MODDIR="${0%/*}"

# Set SELinux policy to allow nfqws to bind to NFQUEUE
# (Magisk handles this automatically in most cases)
if [ -f "$MODDIR/bin/nfqws" ]; then
    chcon u:object_r:system_file:s0 "$MODDIR/bin/nfqws" 2>/dev/null
fi

# Create runtime directories
mkdir -p "$MODDIR/log" 2>/dev/null

# Log boot event
echo "[$(date '+%Y-%m-%d %H:%M:%S')] post-fs-data executed" >> "$MODDIR/log/boot.log"
