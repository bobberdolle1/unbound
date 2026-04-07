/*
 * ============================================================================
 * UnboundTunnelEngine — tvOS Packet Tunnel Engine
 * ============================================================================
 * Adapts the existing tpws engine (from theos/unbound-legacy) for use within
 * the tvOS NEPacketTunnelProvider framework.
 * 
 * This wrapper provides a Swift-callable interface to the C-based DPI bypass
 * engine, managing its lifecycle within the tvOS extension sandbox.
 * ============================================================================
 */

#ifndef UnboundTunnelEngine_h
#define UnboundTunnelEngine_h

#include <stdio.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

/*
 * Configuration for the tunnel engine
 */
typedef struct {
    int port;                    /* Local SOCKS proxy port (default: 1993) */
    const char *bind_addr;       /* Bind address (default: "127.0.0.1") */
    
    /* DPI bypass strategy flags */
    const char *desync_mode;     /* "split", "fake", "fake,split", etc. */
    int desync_pos;              /* Position for desync (default: 2) */
    int desync_repeats;          /* Number of repeats (default: 6) */
    bool autottl;                /* Auto-TTL mode */
    int fake_ttl;                /* TTL for fake packets */
    
    /* Domain filtering */
    const char *hostlist_file;   /* Path to domain list file */
    
    /* Logging */
    int log_level;               /* 0=quiet, 1=normal, 2=verbose */
} tunnel_config_t;

/*
 * Initialize the tunnel engine with the given configuration.
 * Returns 0 on success, < 0 on error.
 * Must be called before tunnel_start().
 */
int tunnel_init(const tunnel_config_t *config);

/*
 * Start the tunnel engine. This begins listening for connections
 * and processing DPI bypass requests.
 * Returns 0 on success, < 0 on error.
 */
int tunnel_start(void);

/*
 * Gracefully stop the tunnel engine.
 * All active connections will be closed.
 */
void tunnel_stop(void);

/*
 * Check if the tunnel is currently running.
 */
bool tunnel_is_running(void);

/*
 * Get the number of active connections.
 */
int tunnel_get_active_connections(void);

/*
 * Get engine version string.
 */
const char* tunnel_get_version(void);

#ifdef __cplusplus
}
#endif

#endif /* UnboundTunnelEngine_h */
