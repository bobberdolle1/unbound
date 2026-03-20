#!/system/bin/sh

killall -9 nfqws_arm64 nfqws_arm 2>/dev/null

iptables -t mangle -F POSTROUTING 2>/dev/null
ip6tables -t mangle -F POSTROUTING 2>/dev/null

rm -f /data/local/tmp/unbound_status.log
rm -f /data/local/tmp/unbound_error.log

echo "Unbound DPI Bypass uninstalled successfully"
