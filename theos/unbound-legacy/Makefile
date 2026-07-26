# ============================================================================
# Unbound Legacy -- DPI/Censorship Bypass for Jailbroken iOS
# Dual-Architecture: ARMv7 (iOS 6.1.3 skeuomorphic) + ARM64 (iOS 15+ modern)
# ============================================================================
# The runtime side of that split is sound: UnboundAppDelegate picks the
# skeuomorphic controller below iOS 7 and the modern one above, so one binary
# genuinely serves both eras.
#
# The build side is not, and it cannot be fixed by editing these two lines:
#
#   1. ARCHS lists armv7 and arm64 while TARGET names SDK 6.1. arm64 did not
#      exist before iOS 7, so the 6.1 SDK has no arm64 slice - that pairing
#      cannot produce the binary it claims to. Conversely Xcode 14 and newer
#      dropped armv7 entirely, so no single modern toolchain builds both. Two
#      builds with two toolchains is the only way:
#
#        armv7 : an Xcode/SDK old enough to still emit armv7 (<= Xcode 13),
#                TARGET = iphone:clang:9.3:6.1
#        arm64 : a current SDK, TARGET = iphone:clang:latest:7.0
#
#      engine/Makefile.tpws already does exactly this for the C engine - two
#      clang invocations with per-arch -miphoneos-version-min, lipo'd together.
#      The Theos side never followed.
#
#   2. control declares Architecture: iphoneos-arm, the *rootful* designation,
#      and layout/ installs to /Applications and /Library/LaunchDaemons. On the
#      rootless jailbreaks that dominate iOS 15+ (Dopamine, palera1n rootless)
#      packages must be iphoneos-arm64 with everything under /var/jb. So the
#      "iOS 15+" half of the claim currently reaches rootful installs only.
#      Theos builds that variant with THEOS_PACKAGE_SCHEME=rootless.
#
# Neither is worth untangling before the engine links at all - see
# engine/Makefile.tpws and the iOS notes in README.md.
#
# Left as-is deliberately: the armv7/iOS 6.1 path is the one that matches this
# TARGET and the rootful layout, and silently "fixing" TARGET would produce a
# package that installs and then fails on the devices it claims to add.

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
