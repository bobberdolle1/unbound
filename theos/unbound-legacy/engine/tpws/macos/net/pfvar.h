#pragma once
#include <sys/socket.h>
#include <netinet/in.h>
#include <netinet/ip.h>

#define DIOCNATLOOK _IOWR('D', 23, struct pfioc_natlook)
enum { PF_INOUT, PF_IN, PF_OUT, PF_FWD };

struct pf_addr {
    union { struct in_addr v4; struct in6_addr v6; uint8_t addr8[16]; uint16_t addr16[8]; uint32_t addr32[4]; } pfa;
#define v4 pfa.v4
#define v6 pfa.v6
};

union pf_state_xport { uint16_t port; uint16_t call_id; uint32_t spi; };

struct pfioc_natlook {
    struct pf_addr saddr, daddr, rsaddr, rdaddr;
    union pf_state_xport sxport, dxport, rsxport, rdxport;
    sa_family_t af;
    uint8_t proto, proto_variant, direction;
};

#define UNBOUND_PF_ANCHOR "com.unbound/redirect"
#define UNBOUND_PF_CONF_FILE "/etc/unbound/pf.conf"
