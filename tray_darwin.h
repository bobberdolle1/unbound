#pragma once

#ifdef __cplusplus
extern "C" {
#endif

void setupDockClickObserver(void);
void showAppWindowNative(void);
void initNativeTray(void);
void setNativeTrayIcon(const void *bytes, int length);
void updateNativeTray(const char *statusText, const char *pingText, int isRunning, int activeProfileIndex, const char **profileNames, int profileCount);

#ifdef __cplusplus
}
#endif
