/* Unbound Legacy -- Darwin/iOS Compatibility Layer for tpws
 * Adapts Linux syscalls (epoll, signalfd, timerfd) to kqueue/Darwin */

#ifndef UNBOUND_DARWIN_COMPAT_H
#define UNBOUND_DARWIN_COMPAT_H

#include <sys/types.h>
#include <sys/event.h>
#include <sys/time.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <unistd.h>
#include <errno.h>
#include <fcntl.h>
#include <pthread.h>
#include <Availability.h>

#define IOS_VERSION_MAJOR __IPHONE_OS_VERSION_MIN_REQUIRED
#if IOS_VERSION_MAJOR < 70000
#define IOS6_LEGACY 1
#else
#define IOS6_LEGACY 0
#endif

/* epoll -> kqueue shim */
#define EPOLLIN  0x001
#define EPOLLOUT 0x004
#define EPOLLERR 0x008
#define EPOLLHUP 0x010
#define EPOLLRDHUP 0x2000
#define EPOLLONESHOT (1 << 30)
#define EPOLL_CTL_ADD 1
#define EPOLL_CTL_DEL 2
#define EPOLL_CTL_MOD 3

typedef union epoll_data { void *ptr; int fd; uint32_t u32; uint64_t u64; } epoll_data_t;
struct epoll_event { uint32_t events; epoll_data_t data; };

static inline int epoll_create(int size) { (void)size; int kq = kqueue(); if(kq<0) return -1; fcntl(kq,F_SETFD,FD_CLOEXEC); return kq; }
static inline int epoll_create1(int flags) { int kq = kqueue(); if(kq<0) return -1; if(flags&1) fcntl(kq,F_SETFD,FD_CLOEXEC); return kq; }

static inline int epoll_ctl(int epfd, int op, int fd, struct epoll_event *event) {
    struct kevent kev; int filter = EVFILT_READ, flags = 0;
    if (epfd<0||fd<0) { errno=EBADF; return -1; }
    switch(op) {
    case EPOLL_CTL_ADD: flags=EV_ADD|EV_ENABLE; if(event&&event->events&EPOLLONESHOT) flags|=EV_ONESHOT;
        EV_SET(&kev,fd,filter,flags,0,0,event?event->data.ptr:NULL); return kevent(epfd,&kev,1,NULL,0,NULL);
    case EPOLL_CTL_DEL: flags=EV_DELETE; EV_SET(&kev,fd,filter,flags,0,0,NULL); return kevent(epfd,&kev,1,NULL,0,NULL);
    case EPOLL_CTL_MOD:
        flags=EV_DELETE; EV_SET(&kev,fd,filter,flags,0,0,NULL); kevent(epfd,&kev,1,NULL,0,NULL);
        flags=EV_ADD|EV_ENABLE; if(event&&event->events&EPOLLONESHOT) flags|=EV_ONESHOT;
        EV_SET(&kev,fd,filter,flags,0,0,event?event->data.ptr:NULL); return kevent(epfd,&kev,1,NULL,0,NULL);
    default: errno=EINVAL; return -1; }
}

static inline int epoll_wait(int epfd, struct epoll_event *events, int maxevents, int timeout) {
    struct timespec ts, *tsp = NULL;
    if (timeout>=0) { ts.tv_sec=timeout/1000; ts.tv_nsec=(timeout%1000)*1000000; tsp=&ts; }
    int nev = kevent(epfd,NULL,0,(struct kevent*)events,maxevents,tsp);
    for(int i=0;i<nev&&i<maxevents;i++) {
        struct kevent *k=(struct kevent*)&events[i];
        events[i].events=0;
        if(k->flags&EV_EOF) events[i].events|=EPOLLHUP|EPOLLRDHUP;
        if(k->filter==EVFILT_READ) events[i].events|=EPOLLIN;
        if(k->filter==EVFILT_WRITE) events[i].events|=EPOLLOUT;
    }
    return nev;
}

#ifndef SO_NOSIGPIPE
#define SO_NOSIGPIPE 0x1022
#endif

#if IOS6_LEGACY
static inline int getentropy(void *buf, size_t buflen) { arc4random_buf(buf, buflen); return 0; }
#endif

#endif
