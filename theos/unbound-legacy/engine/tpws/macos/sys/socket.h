#pragma once
#include_next <sys/socket.h>
#ifndef SO_NOSIGPIPE
#define SO_NOSIGPIPE 0x1022
#endif
#ifndef TCP_KEEPALIVE
#define TCP_KEEPALIVE 0x10
#endif
static inline int ios_socket_set_nosigpipe(int fd) { int on=1; return setsockopt(fd,SOL_SOCKET,SO_NOSIGPIPE,&on,sizeof(on)); }
