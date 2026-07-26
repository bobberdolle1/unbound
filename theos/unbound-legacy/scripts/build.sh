#!/bin/bash
# Unbound Legacy -- Build Script
# Usage: ./build.sh [armv7|arm64|both|clean|install [device_ip]]
set -e
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
THEOS="${THEOS:-$HOME/theos}"

check_deps() {
    echo -e "${CYAN}Checking dependencies...${NC}"
    [ -z "$THEOS" ] || [ ! -d "$THEOS" ] && { echo -e "${RED}THEOS not found at $THEOS${NC}"; exit 1; }
    command -v clang &>/dev/null || { echo -e "${RED}clang not found${NC}"; exit 1; }
    echo -e "${GREEN}OK${NC}"
}

build_engine() {
    echo -e "${CYAN}Building tpws engine ($1)...${NC}"
    cd "$PROJECT_DIR/engine"

    # Makefile.tpws compiles 13 upstream sources (tpws.c, tamper.c, hostlist.c
    # and friends) that this repository does not vendor - only ios_main.c and
    # the epoll shim are present. Fetch them before building.
    #
    # This step used to swallow the failure with `2>/dev/null || echo
    # "Engine build skipped"`, so build.sh reported success and went on to
    # package a .deb containing the tweak but no engine for it to drive.
    if [ ! -f "tpws/tpws.c" ]; then
        echo -e "${RED}Upstream tpws sources are missing.${NC}"
        echo "Fetch them first:"
        echo "    ./scripts/fetch-tpws.sh"
        echo "(GPL-3.0 from https://github.com/bol-van/zapret, so not vendored)"
        exit 1
    fi

    # Even with the sources present this link cannot succeed yet. tpws.h
    # declares tpws_init() and tpws_run_loop(), the entry points ios_main.c
    # and the tvOS engine call, but nothing defines them: upstream exposes a
    # plain main(), which ios_main.c also defines - so the two collide.
    #
    # Finishing the port means splitting upstream's main() into an
    # init/run-loop pair behind that header. Until then, say so instead of
    # emitting a package with no working engine in it.
    if ! grep -rq "tpws_run_loop *(void) *{\|tpws_run_loop(void)$" tpws/*.c 2>/dev/null; then
        echo -e "${RED}The tpws wrapper API is not implemented.${NC}"
        echo "tpws.h declares tpws_init()/tpws_run_loop(), but no source defines"
        echo "them; upstream provides main(), which ios_main.c also defines."
        echo "Porting that into an init/run-loop pair is still outstanding -"
        echo "see the iOS/tvOS notes in README.md."
        exit 1
    fi

    make -f Makefile.tpws "$1" || {
        echo -e "${RED}Engine build failed.${NC}"
        exit 1
    }
    cd "$PROJECT_DIR"
}

build_theos() {
    echo -e "${CYAN}Building Theos project${NC}"
    cd "$PROJECT_DIR"
    export THEOS="$THEOS"
    [ "$1" != "both" ] && export ARCHS="$1" || export ARCHS="armv7 arm64"
    make clean 2>/dev/null || true
    make -j"$(nproc 2>/dev/null || echo 4)"
    echo -e "${GREEN}Theos build complete${NC}"
}

package_deb() {
    echo -e "${CYAN}Packaging DEB...${NC}"
    cd "$PROJECT_DIR"
    make package
    local deb=$(ls -t packages/*.deb 2>/dev/null | head -1)
    [ -n "$deb" ] && echo -e "${GREEN}Package: $deb ($(du -h "$deb" | cut -f1))${NC}" || { echo -e "${RED}No DEB found${NC}"; exit 1; }
}

clean() {
    echo -e "${CYAN}Cleaning...${NC}"
    cd "$PROJECT_DIR" && make clean 2>/dev/null || true
    rm -rf .theos packages obj 2>/dev/null || true
    cd "$PROJECT_DIR/engine" && make -f Makefile.tpws clean 2>/dev/null || true
    echo -e "${GREEN}Clean done${NC}"
}

install_device() {
    local ip="${THEOS_DEVICE_IP:-$1}"
    [ -z "$ip" ] && { echo -e "${RED}No device IP${NC}"; exit 1; }
    echo -e "${CYAN}Installing to $ip...${NC}"
    cd "$PROJECT_DIR" && export THEOS_DEVICE_IP="$ip" && make install
    echo -e "${GREEN}Installed${NC}"
}

case "${1:-both}" in
    clean) clean;;
    armv7|arm64) check_deps; build_engine "$1"; build_theos "$1"; package_deb;;
    both) check_deps; build_engine "tpws-universal"; build_theos "both"; package_deb;;
    install) install_device "$2";;
    *) echo "Usage: $0 [armv7|arm64|both|clean|install [ip]]"; exit 1;;
esac
