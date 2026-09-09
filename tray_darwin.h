#pragma once

#ifdef __cplusplus
extern "C" {
#endif

void setupDockClickObserver(void);
void initNativeTray(void);
void updateNativeTray(const char *statusText, const char *pingText, int isRunning, int activeProfileIndex, const char **profileNames, int profileCount);

#ifdef __cplusplus
}
#endif
