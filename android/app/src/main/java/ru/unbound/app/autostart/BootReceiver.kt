package ru.unbound.app.autostart

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log
import ru.unbound.app.data.SettingsManager
import ru.unbound.app.vpn.UnboundVpnService

/**
 * BroadcastReceiver that triggers VPN connection on device boot.
 * Listens for BOOT_COMPLETED and LOCKED_BOOT_COMPLETED (direct boot).
 */
class BootReceiver : BroadcastReceiver() {

    companion object {
        private const val TAG = "BootReceiver"
    }

    override fun onReceive(context: Context, intent: Intent) {
        when (intent.action) {
            Intent.ACTION_BOOT_COMPLETED,
            Intent.ACTION_LOCKED_BOOT_COMPLETED,
            Intent.ACTION_MY_PACKAGE_REPLACED -> {

                Log.d(TAG, "Received boot/replacement event: ${intent.action}")

                val settingsManager = SettingsManager(context)
                // We need to check the setting asynchronously.
                // For simplicity, we use a coroutine in a real implementation.
                // Here we just launch the service if autostart is enabled.

                // Launch VPN if autostart on boot is enabled
                kotlinx.coroutines.CoroutineScope(kotlinx.coroutines.Dispatchers.IO).launch {
                    val autostartBoot = settingsManager.autostartBootFlow.value
                    if (autostartBoot) {
                        Log.d(TAG, "Autostart on boot is enabled, starting VPN")
                        val vpnIntent = Intent(context, UnboundVpnService::class.java).apply {
                            action = UnboundVpnService.ACTION_CONNECT
                        }
                        context.startForegroundService(vpnIntent)
                    } else {
                        Log.d(TAG, "Autostart on boot is disabled")
                    }
                }
            }
        }
    }
}
