UNBOUND DPI BYPASS - ANDROID MODULE
====================================

REQUIREMENTS:
- Rooted Android device (Magisk or KernelSU)
- ARM64 or ARMv7 architecture
- Android 8.0+ recommended

INSTALLATION:
1. Flash the module ZIP via Magisk/KernelSU Manager
2. Reboot device
3. Module will auto-start on boot

PROFILES:
Edit /data/adb/modules/unbound_dpi_bypass/service.sh
Set UNBOUND_PROFILE to one of:
- ultimate   : All services (Telegram, Discord, YouTube, WhatsApp)
- discord    : Discord voice/video optimized
- youtube    : YouTube QUIC aggressive
- telegram   : Telegram API bypass

LOGS:
- Status: /data/local/tmp/unbound_status.log
- Errors: /data/local/tmp/unbound_error.log

BINARIES REQUIRED (NOT INCLUDED):
Place in module's bin/ folder:
- nfqws_arm64 (for arm64-v8a devices)
- nfqws_arm   (for armeabi-v7a devices)

Compile from: https://github.com/bol-van/zapret
Cross-compile for Android using NDK.

UNINSTALLATION:
Remove module via Magisk/KernelSU Manager or flash uninstall ZIP.
