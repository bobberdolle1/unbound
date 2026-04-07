/* Unbound Legacy -- tpws header for iOS */
#ifndef UNBOUND_TPWS_H
#define UNBOUND_TPWS_H
#ifdef __cplusplus
extern "C" {
#endif
int tpws_run_loop(void);
int tpws_init(const void *config);
void tpws_stop(void);
int tpws_get_active_connections(void);
#ifdef __cplusplus
}
#endif
#endif
