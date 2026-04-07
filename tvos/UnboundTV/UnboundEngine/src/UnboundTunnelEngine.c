/*
 * ============================================================================
 * UnboundTunnelEngine.c — tvOS Packet Tunnel Engine Implementation
 * ============================================================================
 * Adapts the existing tpws engine for use within tvOS NEPacketTunnelProvider.
 * This file bridges the C-based tpws engine with the Swift PacketTunnel code.
 * ============================================================================
 */

#include "UnboundTunnelEngine.h"
#include <stdlib.h>
#include <string.h>
#include <pthread.h>
#include <syslog.h>

/* Include tpws engine */
#include "../../theos/unbound-legacy/engine/tpws/tpws.h"

/*
 * ============================================================================
 * Global state
 * ============================================================================
 */

static volatile int g_tunnel_running = 0;
static pthread_t g_tunnel_thread;
static tunnel_config_t g_config;

/*
 * ============================================================================
 * Signal handler (for graceful shutdown)
 * ============================================================================
 */

static void tunnel_signal_handler(int sig) {
    syslog(LOG_NOTICE, "[unbound-tunnel] Received signal %d, shutting down...", sig);
    g_tunnel_running = 0;
}

/*
 * ============================================================================
 * Thread function — runs the tpws event loop
 * ============================================================================
 */

static void *tunnel_thread_func(void *arg) {
    (void)arg;
    syslog(LOG_NOTICE, "[unbound-tunnel] Engine thread started");

    /* Initialize tpws config from tunnel config */
    tpws_config_t tpws_cfg;
    memset(&tpws_cfg, 0, sizeof(tpws_cfg));
    
    tpws_cfg.port = g_config.port;
    tpws_cfg.socks_mode = 1;  /* SOCKS proxy mode for tvOS */
    tpws_cfg.bind_addr = g_config.bind_addr;
    tpws_cfg.max_connections = 100;
    tpws_cfg.timeout_sec = 30;

    /* Initialize tpws */
    int ret = tpws_init(&tpws_cfg);
    if (ret < 0) {
        syslog(LOG_ERR, "[unbound-tunnel] tpws_init failed: %d", ret);
        return (void *)(intptr_t)ret;
    }

    /* Run the main event loop */
    ret = tpws_run_loop();
    
    if (ret < 0) {
        syslog(LOG_ERR, "[unbound-tunnel] Engine exited with error: %d", ret);
    } else {
        syslog(LOG_NOTICE, "[unbound-tunnel] Engine stopped gracefully");
    }

    return (void *)(intptr_t)ret;
}

/*
 * ============================================================================
 * Public API
 * ============================================================================
 */

int tunnel_init(const tunnel_config_t *config) {
    if (!config) {
        syslog(LOG_ERR, "[unbound-tunnel] tunnel_init: NULL config");
        return -1;
    }

    /* Copy config to global state */
    memcpy(&g_config, config, sizeof(tunnel_config_t));

    syslog(LOG_NOTICE, "[unbound-tunnel] Initializing tunnel engine");
    syslog(LOG_NOTICE, "[unbound-tunnel]   Port: %d", g_config.port);
    syslog(LOG_NOTICE, "[unbound-tunnel]   Bind: %s", g_config.bind_addr ? g_config.bind_addr : "NULL");
    syslog(LOG_NOTICE, "[unbound-tunnel]   Desync: %s", g_config.desync_mode ? g_config.desync_mode : "NULL");

    /* Set up signal handlers */
    struct sigaction sa;
    memset(&sa, 0, sizeof(sa));
    sa.sa_handler = tunnel_signal_handler;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = SA_RESTART;

    sigaction(SIGINT, &sa, NULL);
    sigaction(SIGTERM, &sa, NULL);
    sigaction(SIGHUP, &sa, NULL);
    sigaction(SIGPIPE, &sa, NULL);

    return 0;
}

int tunnel_start(void) {
    if (g_tunnel_running) {
        syslog(LOG_WARNING, "[unbound-tunnel] tunnel_start: already running");
        return -1;
    }

    syslog(LOG_NOTICE, "[unbound-tunnel] Starting tunnel engine");

    g_tunnel_running = 1;

    /* Start engine thread */
    if (pthread_create(&g_tunnel_thread, NULL, tunnel_thread_func, NULL) < 0) {
        syslog(LOG_ERR, "[unbound-tunnel] pthread_create failed: %m");
        g_tunnel_running = 0;
        return -1;
    }

    syslog(LOG_NOTICE, "[unbound-tunnel] Engine thread launched");
    return 0;
}

void tunnel_stop(void) {
    if (!g_tunnel_running) {
        return;
    }

    syslog(LOG_NOTICE, "[unbound-tunnel] Stopping tunnel engine...");

    g_tunnel_running = 0;

    /* Stop tpws */
    tpws_stop();

    /* Wait for thread to finish */
    pthread_join(g_tunnel_thread, NULL);

    syslog(LOG_NOTICE, "[unbound-tunnel] Engine stopped");
}

bool tunnel_is_running(void) {
    return g_tunnel_running != 0;
}

int tunnel_get_active_connections(void) {
    if (!g_tunnel_running) {
        return 0;
    }
    
    /* tpws doesn't expose active connections directly in the legacy API */
    /* In a full implementation, add tpws_get_active_connections() to tpws.h */
    return 0;
}

const char* tunnel_get_version(void) {
    return "1.0.0-tvos";
}
