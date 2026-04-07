# ============================================================================
# Unbound Legacy -- DPI/Censorship Bypass for Jailbroken iOS
# Dual-Architecture: ARMv7 (iOS 6.1.3 skeuomorphic) + ARM64 (iOS 15+ modern)
# ============================================================================

export ARCHS = armv7 arm64
export TARGET = iphone:clang:6.1:6.1

include $(THEOS)/makefiles/common.mk

# 1. TWEAK -- Cydia Substrate / ElleKit hook
TWEAK_NAME = UnboundTweak
UnboundTweak_FILES = unboundTweak/Tweak.xm unboundTweak/UnboundProxyManager.m
UnboundTweak_FRAMEWORKS = UIKit SystemConfiguration Foundation CoreFoundation
UnboundTweak_PRIVATE_FRAMEWORKS = Preferences
UnboundTweak_CFLAGS = -fobjc-arc

# 2. APPLICATION -- Dual UI (Skeuomorphic iOS 6 / Modern iOS 15+)
APPLICATION_NAME = Unbound
Unbound_FILES = $(wildcard unboundApp/*.m) $(wildcard unboundApp/iOS6/*.m) $(wildcard unboundApp/Modern/*.m)
Unbound_FRAMEWORKS = UIKit Foundation CoreGraphics CoreFoundation SystemConfiguration QuartzCore
Unbound_PRIVATE_FRAMEWORKS = Preferences
Unbound_CFLAGS = -fobjc-arc
Unbound_IPHONEOS_DEPLOYMENT_TARGET = 6.1

SUBPROJECTS += UnboundTweak Unbound

include $(THEOS_MAKE_PATH)/aggregate.mk
include $(THEOS_MAKE_PATH)/tweak.mk
include $(THEOS_MAKE_PATH)/application.mk

# Post-build: copy tpws binary into the DEB package
after-package::
	@echo "==> Embedding tpws engine into package..."
	@if [ -f engine/tpws ]; then \
		cp engine/tpws $(THEOS_STAGING_DIR)/usr/local/bin/unbound-tpws; \
		chmod 755 $(THEOS_STAGING_DIR)/usr/local/bin/unbound-tpws; \
		ldid -S $(THEOS_STAGING_DIR)/usr/local/bin/unbound-tpws 2>/dev/null || true; \
	fi
	@echo "==> Setting LaunchDaemon permissions..."
	@if [ -f $(THEOS_STAGING_DIR)/Library/LaunchDaemons/com.unbound.tpws.plist ]; then \
		chmod 644 $(THEOS_STAGING_DIR)/Library/LaunchDaemons/com.unbound.tpws.plist; \
	fi
	@if [ -d $(THEOS_STAGING_DIR)/Applications/Unbound.app ]; then \
		chmod 755 $(THEOS_STAGING_DIR)/Applications/Unbound.app; \
		chmod 755 $(THEOS_STAGING_DIR)/Applications/Unbound.app/Unbound; \
	fi

.PHONY: engine
engine:
	$(MAKE) -f engine/Makefile.tpws

before-all:: engine
