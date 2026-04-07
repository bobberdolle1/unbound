/* epoll-shim implementation for iOS/Darwin -- kqueue-backed */
#include <sys/epoll.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <pthread.h>

int epoll_create(int size) {
    if(size<=0){errno=EINVAL;return -1;}
    int kq=kqueue(); if(kq<0)return -1;
    fcntl(kq,F_SETFD,FD_CLOEXEC);
    return kq;
}

int epoll_create1(int flags) {
    if(flags&~O_CLOEXEC){errno=EINVAL;return -1;}
    int kq=kqueue(); if(kq<0)return -1;
    if(flags&O_CLOEXEC)fcntl(kq,F_SETFD,FD_CLOEXEC);
    return kq;
}

int epoll_ctl(int epfd, int op, int fd, struct epoll_event *event) {
    struct kevent kev; int filter=EVFILT_READ, flags=0;
    if(epfd<0||fd<0){errno=EBADF;return -1;}
    if(event){if(event->events&EPOLLOUT)filter=EVFILT_WRITE; if(event->events&EPOLLONESHOT)flags|=EV_ONESHOT; if(event->events&EPOLLET)flags|=EV_CLEAR;}
    switch(op){
    case EPOLL_CTL_ADD: flags|=EV_ADD|EV_ENABLE; EV_SET(&kev,fd,filter,flags,0,0,event?event->data.ptr:NULL); return kevent(epfd,&kev,1,NULL,0,NULL);
    case EPOLL_CTL_DEL: flags=EV_DELETE; EV_SET(&kev,fd,filter,flags,0,0,NULL); return kevent(epfd,&kev,1,NULL,0,NULL);
    case EPOLL_CTL_MOD:
        flags=EV_DELETE;EV_SET(&kev,fd,filter,flags,0,0,NULL);kevent(epfd,&kev,1,NULL,0,NULL);
        flags=EV_ADD|EV_ENABLE; if(event&&event->events&EPOLLONESHOT)flags|=EV_ONESHOT;
        EV_SET(&kev,fd,filter,flags,0,0,event?event->data.ptr:NULL);return kevent(epfd,&kev,1,NULL,0,NULL);
    default:errno=EINVAL;return -1;}
}

int epoll_wait(int epfd, struct epoll_event *events, int maxevents, int timeout) {
    struct timespec ts,*tsp=NULL;
    if(timeout>=0){ts.tv_sec=timeout/1000;ts.tv_nsec=(timeout%1000)*1000000;tsp=&ts;}
    int nev=kevent(epfd,NULL,0,(struct kevent*)events,maxevents<128?maxevents:128,tsp);
    if(nev<0)return -1;
    for(int i=0;i<nev&&i<maxevents;i++){
        struct kevent*k=(struct kevent*)&events[i];
        uint32_t ev=0;
        if(k->filter==EVFILT_READ)ev|=EPOLLIN;
        if(k->filter==EVFILT_WRITE)ev|=EPOLLOUT;
        if(k->flags&EV_EOF)ev|=EPOLLHUP|EPOLLRDHUP;
        events[i].events=ev;
    }
    return nev;
}

int signalfd(int fd, const sigset_t *mask, int flags) { (void)fd;(void)mask;(void)flags; errno=ENOSYS; return -1; }

int timerfd_create(int clockid, int flags) {
    (void)clockid; int fds[2];
    if(pipe(fds)<0)return -1;
    if(flags&TFD_CLOEXEC){fcntl(fds[0],F_SETFD,FD_CLOEXEC);fcntl(fds[1],F_SETFD,FD_CLOEXEC);}
    return fds[0];
}

int timerfd_settime(int fd, int flags, const struct itimerspec *new_value, struct itimerspec *old_value) {
    (void)flags;(void)old_value; uint64_t exp=1; return write(fd,&exp,sizeof(exp))<0?-1:0;
}

int timerfd_gettime(int fd, struct itimerspec *curr_value) {
    (void)fd; if(curr_value){curr_value->it_value.tv_sec=0;curr_value->it_value.tv_nsec=0;curr_value->it_interval.tv_sec=0;curr_value->it_interval.tv_nsec=0;} return 0;
}

int eventfd(unsigned int initval, int flags) {
    int fds[2]; if(pipe(fds)<0)return -1;
    if(flags&EFD_CLOEXEC){fcntl(fds[0],F_SETFD,FD_CLOEXEC);fcntl(fds[1],F_SETFD,FD_CLOEXEC);}
    uint64_t val=initval; if(write(fds[1],&val,sizeof(val))<0){close(fds[0]);close(fds[1]);return -1;}
    return fds[0];
}
int eventfd_read(int fd, eventfd_t *value) { ssize_t n=read(fd,value,sizeof(*value)); return(n==sizeof(*value))?0:-1; }
int eventfd_write(int fd, eventfd_t value) { ssize_t n=write(fd,&value,sizeof(value)); return(n==sizeof(value))?0:-1; }
