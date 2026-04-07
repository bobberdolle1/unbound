package ru.unbound.app.data

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

/**
 * Manages lists of allowed/disallowed apps and trusted WiFi SSIDs.
 * Stored as comma-separated strings for simplicity.
 */
class AppDataManager(context: Context) {

    private val Context.dataStore: DataStore<Preferences> by preferencesDataStore(name = "app_data")
    private val dataStore = context.dataStore

    // =========================================================================
    // Keys
    // =========================================================================

    private val KEY_ALLOWED_APPS = stringPreferencesKey("allowed_apps")
    private val KEY_DISALLOWED_APPS = stringPreferencesKey("disallowed_apps")
    private val KEY_TRUSTED_WIFI_SSIDS = stringPreferencesKey("trusted_wifi_ssids")
    private val KEY_USE_USAGE_STATS = booleanPreferencesKey("use_usage_stats")

    // =========================================================================
    // Apps (comma-separated package names)
    // =========================================================================

    val allowedAppsFlow: Flow<Set<String>> = dataStore.data
        .map { prefs ->
            (prefs[KEY_ALLOWED_APPS] ?: "").split(",").filter { it.isNotBlank() }.toSet()
        }

    val disallowedAppsFlow: Flow<Set<String>> = dataStore.data
        .map { prefs ->
            (prefs[KEY_DISALLOWED_APPS] ?: "").split(",").filter { it.isNotBlank() }.toSet()
        }

    suspend fun setAllowedApps(packages: Set<String>) {
        dataStore.edit { prefs ->
            prefs[KEY_ALLOWED_APPS] = packages.joinToString(",")
        }
    }

    suspend fun setDisallowedApps(packages: Set<String>) {
        dataStore.edit { prefs ->
            prefs[KEY_DISALLOWED_APPS] = packages.joinToString(",")
        }
    }

    suspend fun addAllowedApp(packageName: String) {
        val current = allowedAppsFlow.value
        setAllowedApps(current + packageName)
    }

    suspend fun removeAllowedApp(packageName: String) {
        val current = allowedAppsFlow.value
        setAllowedApps(current - packageName)
    }

    suspend fun addDisallowedApp(packageName: String) {
        val current = disallowedAppsFlow.value
        setDisallowedApps(current + packageName)
    }

    suspend fun removeDisallowedApp(packageName: String) {
        val current = disallowedAppsFlow.value
        setDisallowedApps(current - packageName)
    }

    // =========================================================================
    // Trusted WiFi SSIDs (comma-separated)
    // =========================================================================

    val trustedWifiSsidsFlow: Flow<Set<String>> = dataStore.data
        .map { prefs ->
            (prefs[KEY_TRUSTED_WIFI_SSIDS] ?: "").split(",").filter { it.isNotBlank() }.toSet()
        }

    suspend fun setTrustedWifiSsids(ssids: Set<String>) {
        dataStore.edit { prefs ->
            prefs[KEY_TRUSTED_WIFI_SSIDS] = ssids.joinToString(",")
        }
    }

    suspend fun addTrustedWifiSsid(ssid: String) {
        val current = trustedWifiSsidsFlow.value
        setTrustedWifiSsids(current + ssid)
    }

    suspend fun removeTrustedWifiSsid(ssid: String) {
        val current = trustedWifiSsidsFlow.value
        setTrustedWifiSsids(current - ssid)
    }

    // =========================================================================
    // Usage Stats Toggle
    // =========================================================================

    val useUsageStatsFlow: Flow<Boolean> = dataStore.data
        .map { prefs -> prefs[KEY_USE_USAGE_STATS] ?: true }

    suspend fun setUseUsageStats(enabled: Boolean) {
        dataStore.edit { prefs ->
            prefs[KEY_USE_USAGE_STATS] = enabled
        }
    }
}
