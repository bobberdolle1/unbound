package ru.unbound.app.autostart

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.wifi.WifiInfo
import android.net.wifi.WifiManager
import android.util.Log
import ru.unbound.app.data.AppDataManager
import ru.unbound.app.data.SettingsManager
import ru.unbound.app.vpn.UnboundVpnService

/**
 * BroadcastReceiver that monitors Wi-Fi state changes.
 * If the device connects to a trusted SSID and autostart on Wi-Fi is enabled,
 * the VPN service is started.
 */
class WifiStateReceiver : BroadcastReceiver() {

    companion object {
        private const val TAG = "WifiStateReceiver"
    }

    override fun onReceive(context: Context, intent: Intent) {
        when (intent.action) {
            WifiManager.NETWORK_STATE_CHANGED_ACTION,
            ConnectivityManager.CONNECTIVITY_ACTION -> {

                val settingsManager = SettingsManager(context)
                val appDataManager = AppDataManager(context)

                kotlinx.coroutines.CoroutineScope(kotlinx.coroutines.Dispatchers.IO).launch {
                    val autostartWifi = settingsManager.autostartWifiFlow.value
                    if (!autostartWifi) {
                        Log.d(TAG, "Autostart on Wi-Fi is disabled")
                        return@launch
                    }

                    val trustedSsids = appDataManager.trustedWifiSsidsFlow.value
                    if (trustedSsids.isEmpty()) {
                        Log.d(TAG, "No trusted SSIDs configured")
                        return@launch
                    }

                    // Get current Wi-Fi SSID
                    val currentSsid = getCurrentWifiSsid(context)
                    if (currentSsid != null && currentSsid in trustedSsids) {
                        Log.d(TAG, "Connected to trusted SSID: $currentSsid, starting VPN")
                        val vpnIntent = Intent(context, UnboundVpnService::class.java).apply {
                            action = UnboundVpnService.ACTION_CONNECT
                        }
                        context.startForegroundService(vpnIntent)
                    } else {
                        Log.d(TAG, "Connected to non-trusted SSID: $currentSsid")
                    }
                }
            }
        }
    }

    /**
     * Gets the current Wi-Fi SSID. Requires location permission on Android 8.1+.
     */
    private fun getCurrentWifiSsid(context: Context): String? {
        return try {
            val wifiManager = context.applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
            val connectionInfo: WifiInfo = wifiManager.connectionInfo
            val ssid = connectionInfo.ssid

            // SSID is returned with quotes (e.g., "MyNetwork"), strip them
            if (ssid.startsWith("\"") && ssid.endsWith("\"")) {
                ssid.substring(1, ssid.length - 1)
            } else if (ssid == "<unknown ssid>") {
                null
            } else {
                ssid
            }
        } catch (e: Exception) {
            Log.e(TAG, "Failed to get Wi-Fi SSID: ${e.message}", e)
            null
        }
    }
}

/**
 * Modern NetworkCallback-based Wi-Fi monitor (API 29+).
 * Use this instead of the broadcast receiver on newer Android versions.
 */
class WifiNetworkMonitor(private val context: Context) {

    companion object {
        private const val TAG = "WifiNetworkMonitor"
    }

    private val connectivityManager = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
    private var networkCallback: ConnectivityManager.NetworkCallback? = null

    fun startMonitoring() {
        val callback = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                super.onAvailable(network)
                val capabilities = connectivityManager.getNetworkCapabilities(network)
                if (capabilities?.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) == true) {
                    checkWifiAndStartVpn()
                }
            }

            override fun onLost(network: Network) {
                super.onLost(network)
                Log.d(TAG, "Network lost")
            }
        }

        networkCallback = callback
        try {
            connectivityManager.registerDefaultNetworkCallback(callback)
            Log.d(TAG, "NetworkCallback registered")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to register NetworkCallback: ${e.message}", e)
        }
    }

    fun stopMonitoring() {
        networkCallback?.let {
            try {
                connectivityManager.unregisterNetworkCallback(it)
            } catch (e: Exception) {
                Log.e(TAG, "Failed to unregister NetworkCallback: ${e.message}", e)
            }
        }
        networkCallback = null
    }

    private fun checkWifiAndStartVpn() {
        kotlinx.coroutines.CoroutineScope(kotlinx.coroutines.Dispatchers.IO).launch {
            val settingsManager = SettingsManager(context)
            val appDataManager = AppDataManager(context)

            val autostartWifi = settingsManager.autostartWifiFlow.value
            if (!autostartWifi) return@launch

            val trustedSsids = appDataManager.trustedWifiSsidsFlow.value
            if (trustedSsids.isEmpty()) return@launch

            val currentSsid = getCurrentWifiSsid()
            if (currentSsid != null && currentSsid in trustedSsids) {
                Log.d(TAG, "Connected to trusted SSID via NetworkCallback: $currentSsid")
                val vpnIntent = Intent(context, UnboundVpnService::class.java).apply {
                    action = UnboundVpnService.ACTION_CONNECT
                }
                context.startForegroundService(vpnIntent)
            }
        }
    }

    private fun getCurrentWifiSsid(): String? {
        return try {
            val wifiManager = context.applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
            val connectionInfo: WifiInfo = wifiManager.connectionInfo
            val ssid = connectionInfo.ssid

            if (ssid.startsWith("\"") && ssid.endsWith("\"")) {
                ssid.substring(1, ssid.length - 1)
            } else if (ssid == "<unknown ssid>") {
                null
            } else {
                ssid
            }
        } catch (e: Exception) {
            Log.e(TAG, "Failed to get Wi-Fi SSID: ${e.message}", e)
            null
        }
    }
}
