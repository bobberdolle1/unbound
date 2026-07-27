#include <jni.h>
#include <android/log.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <errno.h>
#include <pthread.h>

#define LOG_TAG "UnboundNativeTunnel"
#define LOGI(...) __android_log_print(ANDROID_LOG_INFO,  LOG_TAG, __VA_ARGS__)
#define LOGE(...) __android_log_print(ANDROID_LOG_ERROR, LOG_TAG, __VA_ARGS__)
#define LOGD(...) __android_log_print(ANDROID_LOG_DEBUG, LOG_TAG, __VA_ARGS__)

static volatile int  g_tunnel_running = 0;
static int           g_tun_fd         = -1;
static int           g_socks_fd       = -1;
static char          g_socks_host[64] = "127.0.0.1";
static int           g_socks_port     = 1080;

/* -----------------------------------------------------------------------
 * connect_socks5 – connect to the SOCKS5 proxy and return a ready socket fd.
 * Returns -1 on failure.
 * --------------------------------------------------------------------- */
static int connect_socks5(const char *host, int port) {
    int sock = socket(AF_INET, SOCK_STREAM, 0);
    if (sock < 0) { LOGE("socket() failed: %s", strerror(errno)); return -1; }

    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port   = htons((uint16_t)port);
    if (inet_pton(AF_INET, host, &addr.sin_addr) != 1) {
        LOGE("inet_pton failed for %s", host);
        close(sock);
        return -1;
    }
    if (connect(sock, (struct sockaddr *)&addr, sizeof(addr)) != 0) {
        LOGE("connect() to SOCKS5 %s:%d failed: %s", host, port, strerror(errno));
        close(sock);
        return -1;
    }

    /* SOCKS5 handshake – no-auth */
    uint8_t hello[3] = {0x05, 0x01, 0x00};
    if (write(sock, hello, sizeof(hello)) != sizeof(hello)) { close(sock); return -1; }
    uint8_t resp[2] = {0, 0};
    if (read(sock, resp, 2) != 2 || resp[1] != 0x00) { close(sock); return -1; }

    LOGI("SOCKS5 connected to %s:%d", host, port);
    return sock;
}

/* -----------------------------------------------------------------------
 * relay_thread – bridges raw TUN packets <-> SOCKS5 TCP stream.
 * Very minimal: reads a packet from TUN, sends it to SOCKS5, and vice versa.
 * --------------------------------------------------------------------- */
static void *relay_thread(void *arg) {
    (void)arg;
    uint8_t buf[65536];

    int socks_fd = connect_socks5(g_socks_host, g_socks_port);
    if (socks_fd < 0) {
        LOGE("Failed to connect SOCKS5 – relay not started.");
        g_tunnel_running = 0;
        return NULL;
    }
    g_socks_fd = socks_fd;

    LOGI("Relay loop started (tun_fd=%d, socks_fd=%d)", g_tun_fd, socks_fd);

    while (g_tunnel_running) {
        fd_set fds;
        FD_ZERO(&fds);
        FD_SET(g_tun_fd, &fds);
        FD_SET(socks_fd, &fds);
        int maxfd = (g_tun_fd > socks_fd ? g_tun_fd : socks_fd) + 1;

        struct timeval tv = {0, 200000}; /* 200 ms */
        int ret = select(maxfd, &fds, NULL, NULL, &tv);
        if (ret < 0) break;
        if (ret == 0) continue;

        if (FD_ISSET(g_tun_fd, &fds)) {
            ssize_t n = read(g_tun_fd, buf, sizeof(buf));
            if (n > 0) write(socks_fd, buf, (size_t)n);
        }
        if (FD_ISSET(socks_fd, &fds)) {
            ssize_t n = read(socks_fd, buf, sizeof(buf));
            if (n > 0) write(g_tun_fd, buf, (size_t)n);
            else if (n == 0) break; /* proxy closed */
        }
    }

    close(socks_fd);
    g_socks_fd = -1;
    LOGI("Relay loop stopped.");
    return NULL;
}

/* -----------------------------------------------------------------------
 * JNI exports – Kotlin `object NativeEngineBridge` uses static dispatch
 * so the second parameter is `jclass`, not `jobject`.
 * --------------------------------------------------------------------- */

JNIEXPORT jint JNICALL
Java_ru_unbound_app_vpn_NativeEngineBridge_startTunTunnel(
        JNIEnv *env,
        jclass  clazz,   /* object singleton -> static */
        jint    tun_fd,
        jstring socks_host,
        jint    socks_port) {

    if (g_tunnel_running) {
        LOGD("Tunnel already running.");
        return 0;
    }

    const char *host = (*env)->GetStringUTFChars(env, socks_host, NULL);
    strncpy(g_socks_host, host, sizeof(g_socks_host) - 1);
    (*env)->ReleaseStringUTFChars(env, socks_host, host);

    g_socks_port    = (int)socks_port;
    g_tun_fd        = (int)tun_fd;
    g_tunnel_running = 1;

    /* Set TUN FD non-blocking */
    int flags = fcntl(tun_fd, F_GETFL, 0);
    if (flags != -1) fcntl(tun_fd, F_SETFL, flags | O_NONBLOCK);

    pthread_t tid;
    if (pthread_create(&tid, NULL, relay_thread, NULL) != 0) {
        LOGE("pthread_create failed: %s", strerror(errno));
        g_tunnel_running = 0;
        return -1;
    }
    pthread_detach(tid);

    LOGI("TUN relay started (fd=%d -> %s:%d)", tun_fd, g_socks_host, socks_port);
    return 0;
}

JNIEXPORT jint JNICALL
Java_ru_unbound_app_vpn_NativeEngineBridge_stopTunTunnel(
        JNIEnv *env,
        jclass  clazz) {
    (void)env; (void)clazz;
    LOGI("Stopping TUN relay.");
    g_tunnel_running = 0;
    if (g_socks_fd >= 0) { close(g_socks_fd); g_socks_fd = -1; }
    return 0;
}
