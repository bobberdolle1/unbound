#!/bin/sh
# ============================================================================
# Unbound WebOS — Startup script for webosbrew init.d
# Place this file at: /var/lib/webosbrew/init.d/unbound
# This script runs on TV boot and sets up the Unbound service
# ============================================================================

# Wait for network to be ready
# iptables rules fail if network isn't up yet
MAX_WAIT=30
WAITED=0
while [ $WAITED -lt $MAX_WAIT ]; do
  if ping -c 1 -W 2 google.com >/dev/null 2>&1; then
    break
  fi
  sleep 2
  WAITED=$((WAITED + 2))
done

# Create iptables chain (will be populated when app connects)
iptables -N UNBOUND_CHAIN 2>/dev/null

# Start the Unbound management service (Node.js Luna service)
# This service listens for Luna calls from the WebOS app
/media/developer/apps/usr/palm/applications/com.unbound.app/services/unbound-service.sh &

# Log startup
logger "[unbound-webos] Startup script executed (waited ${WAITED}s for network)"
