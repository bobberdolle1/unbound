#!/bin/sh
# ============================================================================
# Unbound WebOS — Luna service wrapper
# This script runs as a background daemon and handles Luna service calls
# from the Enact frontend to manage nfqws and iptables
# ============================================================================

ENGINE_PATH="/media/developer/apps/usr/palm/applications/com.unbound.app/bin/nfqws"
HOSTLIST_PATH="/media/developer/apps/usr/palm/applications/com.unbound.app/lists"
PID_FILE="/var/run/unbound-webos.pid"

# Default profile
PROFILE="default"
QUEUE_NUM=200

# ============================================================================
# Helper functions
# ============================================================================

log() {
  logger "[unbound-service] $1"
}

get_profile_args() {
  case "$1" in
    "default")
      echo "--qnum=${QUEUE_NUM} --dpi-desync=split --dpi-desync-pos=2 --dpi-desync-repeats=6 --hostlist=${HOSTLIST_PATH}/youtube.txt"
      ;;
    "aggressive")
      echo "--qnum=${QUEUE_NUM} --dpi-desync=fake,split --dpi-desync-pos=1,midsld --dpi-desync-repeats=11 --dpi-desync-autottl --fake-ttl=1 --hostlist=${HOSTLIST_PATH}/youtube.txt"
      ;;
    "lite")
      echo "--qnum=${QUEUE_NUM} --dpi-desync=split --dpi-desync-pos=2 --dpi-desync-repeats=3 --hostlist=${HOSTLIST_PATH}/youtube.txt"
      ;;
    *)
      echo "--qnum=${QUEUE_NUM} --dpi-desync=split --dpi-desync-pos=2 --dpi-desync-repeats=6 --hostlist=${HOSTLIST_PATH}/youtube.txt"
      ;;
  esac
}

# ============================================================================
# Engine management
# ============================================================================

start_engine() {
  local profile="${1:-default}"
  
  # Check if already running
  if [ -f "$PID_FILE" ] && kill -0 $(cat "$PID_FILE") 2>/dev/null; then
    log "Engine already running (PID $(cat $PID_FILE))"
    echo '{"returnValue":true,"message":"already running"}'
    return 0
  fi

  log "Starting engine with profile: $profile"
  
  # Get profile arguments
  local args=$(get_profile_args "$profile")
  
  # Start nfqws in background
  $ENGINE_PATH $args &
  local pid=$!
  
  # Save PID
  echo $pid > "$PID_FILE"
  
  log "Engine started (PID $pid)"
  echo "{\"returnValue\":true,\"pid\":$pid}"
  return 0
}

stop_engine() {
  if [ -f "$PID_FILE" ]; then
    local pid=$(cat "$PID_FILE")
    if kill -0 $pid 2>/dev/null; then
      kill $pid
      log "Engine stopped (PID $pid)"
    fi
    rm -f "$PID_FILE"
  fi
  
  # Also kill any remaining nfqws processes
  killall nfqws 2>/dev/null
  
  echo '{"returnValue":true}'
  return 0
}

get_status() {
  if [ -f "$PID_FILE" ] && kill -0 $(cat "$PID_FILE") 2>/dev/null; then
    local pid=$(cat "$PID_FILE")
    echo "{\"returnValue\":true,\"running\":true,\"pid\":$pid}"
  else
    echo '{"returnValue":true,"running":false}'
  fi
  return 0
}

setup_iptables() {
  log "Setting up iptables rules"
  
  # Flush existing rules
  iptables -F UNBOUND_CHAIN 2>/dev/null
  iptables -D OUTPUT -j UNBOUND_CHAIN 2>/dev/null
  iptables -X UNBOUND_CHAIN 2>/dev/null
  
  # Create new chain
  iptables -N UNBOUND_CHAIN
  
  # Route HTTPS traffic to NFQUEUE
  iptables -A UNBOUND_CHAIN -p tcp --dport 443 -j NFQUEUE --queue-num $QUEUE_NUM
  
  # Insert jump rule at the top of OUTPUT chain
  iptables -I OUTPUT -j UNBOUND_CHAIN
  
  log "iptables rules applied"
  echo '{"returnValue":true}'
  return 0
}

cleanup_iptables() {
  log "Cleaning up iptables rules"
  
  iptables -D OUTPUT -j UNBOUND_CHAIN 2>/dev/null
  iptables -F UNBOUND_CHAIN 2>/dev/null
  iptables -X UNBOUND_CHAIN 2>/dev/null
  
  log "iptables rules removed"
  echo '{"returnValue":true}'
  return 0
}

# ============================================================================
# Main loop — listen for Luna service calls via stdin/stdout
# This is a simplified implementation; in production, use the full Luna
# service API via the @webos-service npm package
# ============================================================================

# Write PID file for this service
echo $$ > /var/run/unbound-service.pid

log "Service started"

# In a real implementation, this would register as a Luna service
# and handle JSON-RPC calls. For now, we use a simple command interface
# that the frontend calls via org.webosbrew.hbchannel.service/exec

# The frontend directly calls our management functions via the root
# execution service, so this script acts as a helper library.
# 
# For a full production implementation, convert this to a proper
# Luna service using the @webos-service Node.js package.
