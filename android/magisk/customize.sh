#!/system/bin/sh

SKIPUNZIP=0

ui_print "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
ui_print "  Unbound DPI Bypass v2.4.0"
ui_print "  Ultimate Censorship Circumvention"
ui_print "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

ARCH=$(getprop ro.product.cpu.abi)
ui_print "- Device Architecture: $ARCH"

if [ "$ARCH" != "arm64-v8a" ] && [ "$ARCH" != "armeabi-v7a" ]; then
    ui_print "! Unsupported architecture: $ARCH"
    ui_print "! Only arm64-v8a and armeabi-v7a are supported"
    exit 1
fi

ui_print "- Extracting binaries..."
unzip -o "$ZIPFILE" 'bin/*' -d $MODPATH >&2

ui_print "- Setting permissions..."
set_perm_recursive $MODPATH/bin 0 0 0755 0755

ui_print "- Installation complete!"
ui_print "- Module will activate on next reboot"
ui_print "- Default profile: Ultimate Bypass"
ui_print ""
ui_print "To change profile, edit:"
ui_print "/data/adb/modules/unbound_dpi_bypass/service.sh"
ui_print "and set UNBOUND_PROFILE to:"
ui_print "  ultimate | discord | youtube | telegram"
ui_print "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
