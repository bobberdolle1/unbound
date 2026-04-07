/* Unbound Legacy -- iOS tpws daemon entry point */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <signal.h>
#include <syslog.h>
#include <pthread.h>
#include "tpws.h"
#include "darwin_compat.h"

#define UNBOUND_DEFAULT_PORT 1993
#define UNBOUND_PID_FILE "/var/run/unbound-tpws.pid"

static volatile int g_running = 0;
static pthread_t g_tpws_thread;

static void unbound_signal_handler(int sig) {
    syslog(LOG_NOTICE, "[unbound-tpws] Signal %d, shutting down...", sig);
    g_running = 0;
}

static int write_pid_file(void) {
    FILE *f = fopen(UNBOUND_PID_FILE, "w");
    if (!f) { syslog(LOG_ERR, "[unbound-tpws] Cannot write PID: %m"); return -1; }
    fprintf(f, "%d\n", getpid());
    fclose(f);
    return 0;
}

static void *tpws_thread_func(void *arg) {
    (void)arg;
    syslog(LOG_NOTICE, "[unbound-tpws] Engine thread started");
    int ret = tpws_run_loop();
    syslog(LOG_NOTICE, "[unbound-tpws] Engine stopped (ret=%d)", ret);
    return (void *)(intptr_t)ret;
}

int main(int argc, char **argv) {
    int port = UNBOUND_DEFAULT_PORT;
    int daemonize = 1;
    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--port") == 0 && i + 1 < argc) port = atoi(argv[++i]);
        else if (strcmp(argv[i], "--no-daemon") == 0 || strcmp(argv[i], "-f") == 0) daemonize = 0;
        else if (strcmp(argv[i], "--help") == 0) { printf("Usage: unbound-tpws [--port PORT] [--no-daemon]\n"); return 0; }
    }

    openlog("unbound-tpws", LOG_PID | LOG_NDELAY, LOG_DAEMON);
    syslog(LOG_NOTICE, "[unbound-tpws] Starting Unbound Legacy v1.0.0 port=%d", port);

    int sf = 1;
    setsockopt(STDIN_FILENO, SOL_SOCKET, SO_NOSIGPIPE, &sf, sizeof(sf));

    if (daemonize) {
        pid_t pid = fork();
        if (pid < 0) { syslog(LOG_ERR, "fork failed: %m"); return 1; }
        if (pid > 0) { syslog(LOG_NOTICE, "Daemonized PID %d", pid); closelog(); return 0; }
        setsid();
        int dn = open("/dev/null", O_RDWR);
        if (dn >= 0) { dup2(dn, 0); dup2(dn, 1); dup2(dn, 2); close(dn); }
    }

    write_pid_file();

    struct sigaction sa;
    memset(&sa, 0, sizeof(sa));
    sa.sa_handler = unbound_signal_handler;
    sigemptyset(&sa.sa_mask);
    sigaction(SIGINT, &sa, NULL);
    sigaction(SIGTERM, &sa, NULL);
    sigaction(SIGHUP, &sa, NULL);
    sigaction(SIGPIPE, &sa, NULL);

    g_running = 1;
    if (pthread_create(&g_tpws_thread, NULL, tpws_thread_func, NULL) < 0) {
        syslog(LOG_ERR, "pthread_create failed: %m");
        unlink(UNBOUND_PID_FILE);
        closelog();
        return 1;
    }

    while (g_running) pause();

    pthread_join(g_tpws_thread, NULL);
    unlink(UNBOUND_PID_FILE);
    closelog();
    return 0;
}
