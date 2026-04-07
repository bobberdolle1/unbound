# ============================================================================
# Unbound WebOS — nfqws Cross-Compilation Makefile
# Targets: LG WebOS ARM (armv7a-neon-webos-linux-gnueabi)
# Uses webosbrew NDK toolchain
# ============================================================================

# ---------------------------------------------------------------------------
# Toolchain configuration
# ---------------------------------------------------------------------------
# Set WEBOS_SDK_PATH to your NDK installation (default: /opt/webos-sdk-x86_64)
WEBOS_SDK_PATH ?= /opt/webos-sdk-x86_64/1.0.g

# Source the environment-setup script variables here
# The SDK provides:
#   - arm-webos-linux-gnueabi-gcc (cross-compiler)
#   - OEToolchainConfig.cmake (CMake toolchain)
CC = $(WEBOS_SDK_PATH)/sysroots/x86_64-webossdk-linux/usr/bin/arm-webos-linux-gnueabi/arm-webos-linux-gnueabi-gcc
STRIP = $(WEBOS_SDK_PATH)/sysroots/x86_64-webossdk-linux/usr/bin/arm-webos-linux-gnueabi/arm-webos-linux-gnueabi-strip
AR = $(WEBOS_SDK_PATH)/sysroots/x86_64-webossdk-linux/usr/bin/arm-webos-linux-gnueabi/arm-webos-linux-gnueabi-ar

# SDK sysroot for dependencies
SDK_SYSROOT = $(WEBOS_SDK_PATH)/sysroots/armv7a-neon-webos-linux-gnueabi

# ---------------------------------------------------------------------------
# nfqws source (from bol-van/zapret)
# ---------------------------------------------------------------------------
ZAPRET_SRC ?= ../../zapret
NFQ_SRC = $(ZAPRET_SRC)/nfq

# ---------------------------------------------------------------------------
# Compiler flags (optimized for WebOS ARM hardware)
# ---------------------------------------------------------------------------
OPTIMIZE ?= -Os

CFLAGS = -std=gnu99 $(OPTIMIZE) -flto=auto -ffunction-sections -fdata-sections \
         --sysroot=$(SDK_SYSROOT) \
         -march=armv7-a -mtune=cortex-a9 -mfpu=neon -mfloat-abi=softfp \
         -DLINUX

LDFLAGS = -flto=auto -Wl,--gc-sections --sysroot=$(SDK_SYSROOT)
LIBS = -lz -lnetfilter_queue -lnfnetlink -lmnl

# Source files (same as zapret nfq Makefile)
SRC_FILES = $(wildcard $(NFQ_SRC)/*.c) $(wildcard $(NFQ_SRC)/crypto/*.c)

# ---------------------------------------------------------------------------
# Build targets
# ---------------------------------------------------------------------------
BUILD_DIR = ./build
TARGET = $(BUILD_DIR)/nfqws

all: deps $(TARGET)

# Build nfqws binary for WebOS
$(TARGET): $(SRC_FILES) | $(BUILD_DIR)
	$(CC) -s $(CFLAGS) -o $@ $^ $(LIBS) $(LDFLAGS)
	$(STRIP) $@
	@echo "==> Built nfqws for WebOS (armv7a-neon)"

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# ---------------------------------------------------------------------------
# Dependencies (must be cross-compiled for WebOS first)
# ---------------------------------------------------------------------------
# These libraries need to be built using the same WebOS NDK toolchain
# Download and build each one:
#   1. libnfnetlink
#   2. libmnl
#   3. libnetfilter_queue
# ---------------------------------------------------------------------------
DEPS_DIR = $(BUILD_DIR)/deps
LIBNFTL = $(DEPS_DIR)/lib/libnetfilter_queue.a

deps: $(LIBNFTL)

$(LIBNFTL): | $(DEPS_DIR)
	@echo "==> Building dependencies for WebOS..."
	# libnfnetlink
	cd $(DEPS_DIR) && \
	  wget -q https://www.netfilter.org/projects/libnfnetlink/files/libnfnetlink-1.0.2.tar.bz2 && \
	  tar xjf libnfnetlink-1.0.2.tar.bz2 && \
	  cd libnfnetlink-1.0.2 && \
	  ./configure --host=arm-webos-linux-gnueabi \
	    --prefix=$(SDK_SYSROOT)/usr \
	    CC="$(CC)" \
	    CFLAGS="$(CFLAGS)" \
	    LDFLAGS="$(LDFLAGS)" && \
	  make -j$$(nproc) && make install

	# libmnl
	cd $(DEPS_DIR) && \
	  wget -q https://www.netfilter.org/projects/libmnl/files/libmnl-1.0.5.tar.bz2 && \
	  tar xjf libmnl-1.0.5.tar.bz2 && \
	  cd libmnl-1.0.5 && \
	  ./configure --host=arm-webos-linux-gnueabi \
	    --prefix=$(SDK_SYSROOT)/usr \
	    CC="$(CC)" \
	    CFLAGS="$(CFLAGS)" \
	    LDFLAGS="$(LDFLAGS)" && \
	  make -j$$(nproc) && make install

	# libnetfilter_queue
	cd $(DEPS_DIR) && \
	  wget -q https://www.netfilter.org/projects/libnetfilter_queue/files/libnetfilter_queue-1.0.5.tar.bz2 && \
	  tar xjf libnetfilter_queue-1.0.5.tar.bz2 && \
	  cd libnetfilter_queue-1.0.5 && \
	  ./configure --host=arm-webos-linux-gnueabi \
	    --prefix=$(SDK_SYSROOT)/usr \
	    CC="$(CC)" \
	    CFLAGS="$(CFLAGS)" \
	    LDFLAGS="$(LDFLAGS)" && \
	  make -j$$(nproc) && make install

	@echo "==> Dependencies built and installed to SDK sysroot"

$(DEPS_DIR):
	mkdir -p $(DEPS_DIR)

# ---------------------------------------------------------------------------
# CMake alternative build (if preferred)
# ---------------------------------------------------------------------------
cmake-build: | $(BUILD_DIR)
	cmake -B $(BUILD_DIR) \
	  -DCMAKE_TOOLCHAIN_FILE=$(WEBOS_SDK_PATH)/sysroots/x86_64-webossdk-linux/usr/share/cmake/OEToolchainConfig.cmake \
	  -DCMAKE_C_COMPILER=$(CC) \
	  -DCMAKE_SYSROOT=$(SDK_SYSROOT) \
	  -DCMAKE_INSTALL_PREFIX=$(SDK_SYSROOT)/usr \
	  -S $(NFQ_SRC)
	cmake --build $(BUILD_DIR) -j$$(nproc)

# ---------------------------------------------------------------------------
# Package for webosbrew deployment
# ---------------------------------------------------------------------------
package: all
	@echo "==> Packaging nfqws for WebOS..."
	cp $(TARGET) ../native/nfqws/nfqws
	chmod +x ../native/nfqws/nfqws
	@echo "==> Copied nfqws to app bundle"

# ---------------------------------------------------------------------------
# Clean
# ---------------------------------------------------------------------------
clean:
	rm -rf $(BUILD_DIR)
	rm -f ../native/nfqws/nfqws

.PHONY: all deps cmake-build package clean
