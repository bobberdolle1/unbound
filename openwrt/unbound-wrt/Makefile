#
# Copyright (C) 2024 Unbound-WRT Project
#
# This is free software, licensed under the GPL v3 License.
#

include $(TOPDIR)/rules.mk

PKG_NAME:=nfqws-unbound
PKG_VERSION:=2024.04.01
PKG_RELEASE:=1

PKG_SOURCE_PROTO:=git
PKG_SOURCE_URL:=https://github.com/bol-van/zapret.git
PKG_SOURCE_VERSION:=master
PKG_MIRROR_HASH:=skip

PKG_LICENSE:=GPL-3.0-only
PKG_LICENSE_FILES:=LICENSE
PKG_MAINTAINER:=Unbound-WRT Project

PKG_BUILD_DIR:=$(BUILD_DIR)/zapret
PKG_BUILD_PARALLEL:=1

include $(INCLUDE_DIR)/package.mk

define Package/nfqws-unbound
  SECTION:=net
  CATEGORY:=Network
  SUBMENU:=Web Servers/Proxies
  TITLE:=NFQueue userspace wrapper (zapret nfqws)
  URL:=https://github.com/bol-van/zapret
  DEPENDS:=+libnetfilter-queue +libnetfilter-conntrack +libpthread
endef

define Package/nfqws-unbound/description
  A lightweight NFQUEUE userspace daemon from the zapret project.
  Used for DPI/censorship bypass at the router level. Handles packet
  mangling and re-injection for transparent proxy operation on LAN.
endef

TARGET_CFLAGS += $(FPIC) -Os -ffunction-sections -fdata-sections
TARGET_LDFLAGS += -Wl,--gc-sections -s

define Build/Compile
	$(MAKE) $(PKG_JOBS) -C $(PKG_BUILD_DIR)/nfqws \
		CC="$(TARGET_CC)" \
		CFLAGS="$(TARGET_CFLAGS) $(TARGET_CPPFLAGS)" \
		LDFLAGS="$(TARGET_LDFLAGS)" \
		LDLIBS="-lnetfilter_queue -lnetfilter_conntrack" \
		nfqws
endef

define Package/nfqws-unbound/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/nfqws/nfqws $(1)/usr/bin/
	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_DATA) ./files/etc/config/unbound $(1)/etc/config/unbound
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/etc/init.d/unbound $(1)/etc/init.d/unbound
	$(INSTALL_DIR) $(1)/etc/nftables.d
	$(INSTALL_DATA) ./files/etc/nftables.d/90-unbound-wrt.nft $(1)/etc/nftables.d/90-unbound-wrt.nft
endef

$(eval $(call BuildPackage,nfqws-unbound))
