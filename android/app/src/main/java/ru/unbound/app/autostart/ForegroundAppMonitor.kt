package ru.unbound.app.autostart

import android.app.UsageStats
import android.app.UsageStatsManager
import android.app.usage.UsageEvents
import android.content.Context
import android.content.Intent
import android.util.Log
import kotlinx.coroutines.*
import ru.unbound.app.data.AppDataManager
import ru.unbound.app.data.SettingsManager
import ru.unbound.app.vpn.UnboundVpnService
import java.util.*

/**
 * Monitors foreground app changes using UsageStatsManager.
 * When a monitored app comes to the foreground, triggers the VPN.
 */
class ForegroundAppMonitor(private val context: Context) {

    companion object {
        private const val TAG = "ForegroundAppMonitor"
        private const val POLL_INTERVAL_MS = 5000L // Check every 5 seconds
    }

    private val usageStatsManager = context.getSystemService(Context.USAGE_STATS_SERVICE) as UsageStatsManager
    private var monitorJob: Job? = null
    private var lastForegroundApp: String? = null

    /**
     * Starts polling UsageStats to detect foreground app changes.
     */
    fun startMonitoring() {
        if (monitorJob?.isActive == true) {
            Log.w(TAG, "Already monitoring")
            return
        }

        monitorJob = CoroutineScope(Dispatchers.IO + SupervisorJob()).launch {
            while (isActive) {
                try {
                    checkForegroundApp()
                    delay(POLL_INTERVAL_MS)
                } catch (e: Exception) {
                    Log.e(TAG, "Error in monitoring loop: ${e.message}", e)
                }
            }
        }

        Log.d(TAG, "Foreground app monitoring started")
    }

    /**
     * Stops the monitoring coroutine.
     */
    fun stopMonitoring() {
        monitorJob?.cancel()
        monitorJob = null
        Log.d(TAG, "Foreground app monitoring stopped")
    }

    /**
     * Checks the current foreground app and starts VPN if it matches monitored apps.
     */
    private suspend fun checkForegroundApp() {
        val settingsManager = SettingsManager(context)
        val appDataManager = AppDataManager(context)

        // Check if autostart on apps is enabled
        val autostartApps = settingsManager.autostartAppsFlow.value
        if (!autostartApps) return

        // Get the current foreground app
        val foregroundApp = getForegroundApp() ?: return

        // Check if it's the same as last time (avoid duplicate starts)
        if (foregroundApp == lastForegroundApp) return

        lastForegroundApp = foregroundApp

        // Get monitored apps list
        val allowedApps = appDataManager.allowedAppsFlow.value
        val disallowedApps = appDataManager.disallowedAppsFlow.value

        // In "include only" mode, start VPN if app is in allowed list
        val splitMode = settingsManager.splitTunnelModeFlow.value
        if (splitMode == 2 && foregroundApp in allowedApps) {
            Log.d(TAG, "Foreground app $foregroundApp is in allowed list, starting VPN")
            startVpn()
        }

        // In "exclude" mode, we'd stop VPN if app is in disallowed list
        // (This depends on your business logic)
    }

    /**
     * Gets the package name of the current foreground app.
     */
    private fun getForegroundApp(): String? {
        val endTime = System.currentTimeMillis()
        val startTime = endTime - 60000 // Look back 1 minute

        // Method 1: UsageEvents (more reliable for recent events)
        val usageEvents = usageStatsManager.queryEvents(startTime, endTime)
        val event = UsageEvents.Event()
        var lastForegroundPackage: String? = null

        while (usageEvents.hasNextEvent()) {
            usageEvents.getNextEvent(event)
            if (event.eventType == UsageEvents.Event.MOVE_TO_FOREGROUND) {
                lastForegroundPackage = event.packageName
            }
        }

        if (lastForegroundPackage != null) {
            return lastForegroundPackage
        }

        // Method 2: UsageStats (fallback)
        val usageStatsList: List<UsageStats> = usageStatsManager.queryUsageStats(
            UsageStatsManager.INTERVAL_DAILY,
            startTime,
            endTime
        )

        val mostRecent = usageStatsList
            .filter { it.lastTimeUsed > 0 }
            .maxByOrNull { it.lastTimeUsed }

        return mostRecent?.packageName
    }

    private fun startVpn() {
        val intent = Intent(context, UnboundVpnService::class.java).apply {
            action = UnboundVpnService.ACTION_CONNECT
        }
        context.startForegroundService(intent)
    }
}
