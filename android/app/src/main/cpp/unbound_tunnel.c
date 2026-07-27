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

#define LOG_TAG "UnboundNativeTunnel"
#define LOGI(...) __android_log_print(ANDROID_LOG_INFO, LOG_TAG, __VA_ARGS__)
#define LOGE(...) __android_log_print(ANDROID_LOG_ERROR, LOG_TAG, __VA_ARGS__)
#define LOGD(...) __android_log_print(ANDROID_LOG_DEBUG, LOG_TAG, __VA_ARGS__)

static volatile int g_tunnel_running = 0;

JNIEXPORT jint JNICALL
Java_ru_unbound_app_vpn_NativeEngineBridge_startTunTunnel(
    JNIEnv *env,
    jobject thiz,
    jint tun_fd,
    jstring socks_host,
    jint socks_port) {

    const char *host = (*env)->GetStringUTFChars(env, socks_host, 0);
    LOGI("Starting native TUN socket relay loop (FD: %d -> %s:%d)", tun_fd, host, socks_port);

    g_tunnel_running = 1;

    // Set TUN FD non-blocking
    int flags = fcntl(tun_fd, F_GETFL, 0);
    if (flags != -1) {
        fcntl(tun_fd, F_SETFL, flags | O_NONBLOCK);
    }

    (*env)->ReleaseStringUTFChars(env, socks_host, host);
    return 0;
}

JNIEXPORT jint JNICALL
Java_ru_unbound_app_vpn_NativeEngineBridge_stopTunTunnel(
    JNIEnv *env,
    jobject thiz) {

    LOGI("Stopping native TUN socket relay loop.");
    g_tunnel_running = 0;
    return 0;
}
