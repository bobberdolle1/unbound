#!/system/bin/sh
# ============================================================================
# Unbound Core — iptables Cleanup Script (standalone)
# ============================================================================

echo "Removing Unbound iptables rules..."

iptables -t mangle -F UNBOUND_OUTPUT 2>/dev/null
iptables -t mangle -F UNBOUND_FORWARD 2>/dev/null
iptables -t mangle -D OUTPUT -j UNBOUND_OUTPUT 2>/dev/null
iptables -t mangle -D FORWARD -j UNBOUND_FORWARD 2>/dev/null
iptables -t mangle -X UNBOUND_OUTPUT 2>/dev/null
iptables -t mangle -X UNBOUND_FORWARD 2>/dev/null

ip6tables -t mangle -F UNBOUND_OUTPUT6 2>/dev/null
ip6tables -t mangle -D OUTPUT -j UNBOUND_OUTPUT6 2>/dev/null
ip6tables -t mangle -X UNBOUND_OUTPUT6 2>/dev/null

echo "✓ Cleanup complete"
