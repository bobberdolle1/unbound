package ru.unbound.app.data

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import ru.unbound.app.ui.theme.AppTheme

/**
 * Centralized preferences manager using DataStore.
 * Persists theme, autostart rules, proxy settings, etc.
 */
class SettingsManager(context: Context) {

    private val Context.dataStore: DataStore<Preferences> by preferencesDataStore(name = "settings")
    private val dataStore = context.dataStore

    // =========================================================================
    // Preference Keys
    // =========================================================================

    private val KEY_THEME = stringPreferencesKey("theme")
    private val KEY_VPN_CONNECTED = booleanPreferencesKey("vpn_connected")
    private val KEY_PROXY_HOST = stringPreferencesKey("proxy_host")
    private val KEY_PROXY_PORT = intPreferencesKey("proxy_port")
    private val KEY_DNS_SERVER = stringPreferencesKey("dns_server")
    private val KEY_AUTOSTART_BOOT = booleanPreferencesKey("autostart_boot")
    private val KEY_AUTOSTART_WIFI = booleanPreferencesKey("autostart_wifi")
    private val KEY_AUTOSTART_APPS = booleanPreferencesKey("autostart_apps")
    private val KEY_SPLIT_TUNNEL_MODE = intPreferencesKey("split_tunnel_mode")
    private val KEY_ROOT_MODULE_ENABLED = booleanPreferencesKey("root_module_enabled")

    // =========================================================================
    // Theme
    // =========================================================================

    val themeFlow: Flow<AppTheme> = dataStore.data
        .map { prefs ->
            AppTheme.fromString(prefs[KEY_THEME] ?: AppTheme.DARK.name)
        }

    suspend fun setTheme(theme: AppTheme) {
        dataStore.edit { prefs ->
            prefs[KEY_THEME] = theme.name
        }
    }

    // =========================================================================
    // VPN State
    // =========================================================================

    val vpnConnectedFlow: Flow<Boolean> = dataStore.data
        .map { prefs -> prefs[KEY_VPN_CONNECTED] ?: false }

    suspend fun setVpnConnected(connected: Boolean) {
        dataStore.edit { prefs ->
            prefs[KEY_VPN_CONNECTED] = connected
        }
    }

    // =========================================================================
    // Proxy Settings
    // =========================================================================

    val proxyHostFlow: Flow<String> = dataStore.data
        .map { prefs -> prefs[KEY_PROXY_HOST] ?: "127.0.0.1" }

    val proxyPortFlow: Flow<Int> = dataStore.data
        .map { prefs -> prefs[KEY_PROXY_PORT] ?: 1080 }

    suspend fun setProxySettings(host: String, port: Int) {
        dataStore.edit { prefs ->
            prefs[KEY_PROXY_HOST] = host
            prefs[KEY_PROXY_PORT] = port
        }
    }

    // =========================================================================
    // DNS Settings
    // =========================================================================

    val dnsServerFlow: Flow<String> = dataStore.data
        .map { prefs -> prefs[KEY_DNS_SERVER] ?: "" }

    suspend fun setDnsServer(dns: String) {
        dataStore.edit { prefs ->
            prefs[KEY_DNS_SERVER] = dns
        }
    }

    // =========================================================================
    // Autostart Rules
    // =========================================================================

    val autostartBootFlow: Flow<Boolean> = dataStore.data
        .map { prefs -> prefs[KEY_AUTOSTART_BOOT] ?: false }

    val autostartWifiFlow: Flow<Boolean> = dataStore.data
        .map { prefs -> prefs[KEY_AUTOSTART_WIFI] ?: false }

    val autostartAppsFlow: Flow<Boolean> = dataStore.data
        .map { prefs -> prefs[KEY_AUTOSTART_APPS] ?: false }

    suspend fun setAutostartBoot(enabled: Boolean) {
        dataStore.edit { prefs -> prefs[KEY_AUTOSTART_BOOT] = enabled }
    }

    suspend fun setAutostartWifi(enabled: Boolean) {
        dataStore.edit { prefs -> prefs[KEY_AUTOSTART_WIFI] = enabled }
    }

    suspend fun setAutostartApps(enabled: Boolean) {
        dataStore.edit { prefs -> prefs[KEY_AUTOSTART_APPS] = enabled }
    }

    // =========================================================================
    // Split Tunneling Mode
    // =========================================================================
    // 0 = All apps, 1 = Exclude selected, 2 = Include only selected

    val splitTunnelModeFlow: Flow<Int> = dataStore.data
        .map { prefs -> prefs[KEY_SPLIT_TUNNEL_MODE] ?: 0 }

    suspend fun setSplitTunnelMode(mode: Int) {
        dataStore.edit { prefs ->
            prefs[KEY_SPLIT_TUNNEL_MODE] = mode
        }
    }

    // =========================================================================
    // Root Module
    // =========================================================================

    val rootModuleEnabledFlow: Flow<Boolean> = dataStore.data
        .map { prefs -> prefs[KEY_ROOT_MODULE_ENABLED] ?: false }

    suspend fun setRootModuleEnabled(enabled: Boolean) {
        dataStore.edit { prefs ->
            prefs[KEY_ROOT_MODULE_ENABLED] = enabled
        }
    }

    // =========================================================================
    // All Settings (for settings screen)
    // =========================================================================

    data class AppSettings(
        val theme: AppTheme,
        val vpnConnected: Boolean,
        val proxyHost: String,
        val proxyPort: Int,
        val dnsServer: String,
        val autostartBoot: Boolean,
        val autostartWifi: Boolean,
        val autostartApps: Boolean,
        val splitTunnelMode: Int,
        val rootModuleEnabled: Boolean
    )

    val settingsFlow: Flow<AppSettings> = dataStore.data.map { prefs ->
        AppSettings(
            theme = AppTheme.fromString(prefs[KEY_THEME] ?: AppTheme.DARK.name),
            vpnConnected = prefs[KEY_VPN_CONNECTED] ?: false,
            proxyHost = prefs[KEY_PROXY_HOST] ?: "127.0.0.1",
            proxyPort = prefs[KEY_PROXY_PORT] ?: 1080,
            dnsServer = prefs[KEY_DNS_SERVER] ?: "",
            autostartBoot = prefs[KEY_AUTOSTART_BOOT] ?: false,
            autostartWifi = prefs[KEY_AUTOSTART_WIFI] ?: false,
            autostartApps = prefs[KEY_AUTOSTART_APPS] ?: false,
            splitTunnelMode = prefs[KEY_SPLIT_TUNNEL_MODE] ?: 0,
            rootModuleEnabled = prefs[KEY_ROOT_MODULE_ENABLED] ?: false
        )
    }
}
