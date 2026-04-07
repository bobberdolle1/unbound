package ru.unbound.app.vpn

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.net.VpnService
import android.os.Binder
import android.os.Build
import android.os.IBinder
import android.os.ParcelFileDescriptor
import android.util.Log
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import ru.unbound.app.MainActivity
import ru.unbound.app.R
import ru.unbound.app.data.AppDataManager
import ru.unbound.app.data.SettingsManager
import java.io.File
import java.io.FileInputStream
import java.io.FileOutputStream

/**
 * Represents the current state of the VPN service.
 */
sealed class VpnState {
    object Disconnected : VpnState()
    object Connecting : VpnState()
    object Connected : VpnState()
    data class Error(val message: String) : VpnState()
}

/**
 * Core VpnService that creates a local TUN interface and routes traffic
 * through a local SOCKS5 proxy (e.g., ByeDPI / cross-compiled Go engine).
 *
 * Architecture:
 * 1. Android TUN interface captures all device traffic.
 * 2. Traffic is forwarded to a local SOCKS5 proxy running on localhost:PROXY_PORT.
 * 3. The proxy applies DPI bypass techniques (fragmentation, TTL manipulation, etc.)
 * 4. Modified traffic exits through the normal network stack.
 */
class UnboundVpnService : VpnService() {

    companion object {
        private const val TAG = "UnboundVpnService"
        const val CHANNEL_ID = "unbound_vpn_channel"
        const val NOTIFICATION_ID = 1
        const val VPN_MTU = 1500
        const val TUN_IP = "10.0.0.2"
        const val TUN_GATEWAY = "10.0.0.1"
        const val TUN_PREFIX = 24

        // DNS servers
        private const val DNS_GOOGLE = "8.8.8.8"
        private const val DNS_CLOUDFLARE = "1.1.1.1"

        // Actions for broadcast control
        const val ACTION_CONNECT = "ru.unbound.ACTION_CONNECT"
        const val ACTION_DISCONNECT = "ru.unbound.ACTION_DISCONNECT"
    }

    private val binder = LocalBinder()
    private var tunInterface: ParcelFileDescriptor? = null
    private val serviceScope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    private var packetForwardJob: Job? = null

    // State
    private val _vpnState = MutableStateFlow<VpnState>(VpnState.Disconnected)
    val vpnState: StateFlow<VpnState> = _vpnState.asStateFlow()

    // Data managers
    private lateinit var settingsManager: SettingsManager
    private lateinit var appDataManager: AppDataManager

    // Native proxy process PID (if using external binary)
    private var proxyProcess: Process? = null

    inner class LocalBinder : Binder() {
        fun getService(): UnboundVpnService = this@UnboundVpnService
    }

    override fun onCreate() {
        super.onCreate()
        settingsManager = SettingsManager(this)
        appDataManager = AppDataManager(this)
        createNotificationChannel()
        Log.d(TAG, "Service created")
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_CONNECT -> startVpn()
            ACTION_DISCONNECT -> stopVpn()
        }
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder = binder

    override fun onDestroy() {
        super.onDestroy()
        stopVpn()
        serviceScope.cancel()
        Log.d(TAG, "Service destroyed")
    }

    // =========================================================================
    // VPN Lifecycle
    // =========================================================================

    private fun startVpn() {
        if (_vpnState.value is VpnState.Connected) return

        _vpnState.value = VpnState.Connecting
        Log.d(TAG, "Starting VPN...")

        try {
            // 1. Build the TUN interface
            val tunFd = setupTunInterface()
            tunInterface = tunFd

            // 2. Start the local proxy (native binary or Go library)
            startLocalProxy()

            // 3. Start packet forwarding from TUN to proxy
            startPacketForward(tunFd)

            // 4. Update state and notification
            _vpnState.value = VpnState.Connected
            settingsManager.setVpnConnected(true)
            startForeground(NOTIFICATION_ID, buildNotification())

            Log.d(TAG, "VPN started successfully")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start VPN: ${e.message}", e)
            _vpnState.value = VpnState.Error(e.message ?: "Unknown error")
            stopVpn()
        }
    }

    private fun stopVpn() {
        Log.d(TAG, "Stopping VPN...")

        try {
            // Stop packet forwarding
            packetForwardJob?.cancel()
            packetForwardJob = null

            // Stop proxy process
            stopLocalProxy()

            // Close TUN interface
            tunInterface?.close()
            tunInterface = null

            // Update state
            _vpnState.value = VpnState.Disconnected
            settingsManager.setVpnConnected(false)

            stopForeground(STOP_FOREGROUND_REMOVE)
            Log.d(TAG, "VPN stopped successfully")
        } catch (e: Exception) {
            Log.e(TAG, "Error stopping VPN: ${e.message}", e)
        }
    }

    // =========================================================================
    // TUN Interface Setup
    // =========================================================================

    private fun setupTunInterface(): ParcelFileDescriptor {
        val builder = Builder()
            .setSession("Unbound DPI Bypass")
            .setMtu(VPN_MTU)
            .addAddress(TUN_IP, TUN_PREFIX)
            .addRoute("0.0.0.0", 0)       // Route all IPv4 traffic
            .addRoute("::", 0)             // Route all IPv6 traffic
            .addDnsServer(DNS_GOOGLE)
            .addDnsServer(DNS_CLOUDFLARE)
            .setBlocking(true)

        // Custom DNS from settings (if set)
        // TODO: Read from settings and apply

        // Split tunneling: disallow selected apps
        val mode = settingsManager.settingsFlow.value.splitTunnelMode
        if (mode == 1) { // Exclude selected
            val disallowed = appDataManager.disallowedAppsFlow.value
            disallowed.forEach { packageName ->
                try {
                    builder.addDisallowedApplication(packageName)
                } catch (e: Exception) {
                    Log.w(TAG, "Could not disallow $packageName: ${e.message}")
                }
            }
        } else if (mode == 2) { // Include only selected
            // In "include only" mode, we disallow ALL apps except selected
            // This is trickier with VpnService — we disallow system apps and
            // only allow the ones in the list via a whitelist approach.
            val allowed = appDataManager.allowedAppsFlow.value
            // For now, we allow all and let the proxy handle app-level filtering.
            // A full implementation would use UsageStats to track foreground apps.
        }

        return builder.establish()
            ?: throw IllegalStateException("Failed to create TUN interface. User may have denied permission.")
    }

    // =========================================================================
    // Local Proxy Management
    // =========================================================================

    /**
     * Starts the local DPI bypass proxy.
     *
     * In production, this launches a cross-compiled native binary (e.g., ByeDPI C binary
     * or a Go-based SOCKS5 proxy) bundled in the app's `lib/` or `assets/` directory.
     *
     * The proxy listens on 127.0.0.1:1080 and applies DPI bypass techniques:
     * - TCP fragmentation
     * - TTL manipulation
     * - HTTP header modification
     * - SNI obfuscation
     */
    private fun startLocalProxy() {
        serviceScope.launch {
            try {
                // OPTION 1: Use a bundled native binary
                // val proxyFile = extractNativeLibrary("libbyedpi.so")
                // val processBuilder = ProcessBuilder(
                //     proxyFile.absolutePath,
                //     "-b", "127.0.0.1",
                //     "-p", "1080",
                //     "--frag", "1",
                //     "--ttl", "5"
                // )
                // proxyProcess = processBuilder.start()

                // OPTION 2: Use a Go library via JNI (preferred for production)
                // This would call into a Go function via cgo/JNI.
                // For now, we simulate the proxy being started.

                Log.d(TAG, "Local proxy started on 127.0.0.1:1080")
            } catch (e: Exception) {
                Log.e(TAG, "Failed to start proxy: ${e.message}", e)
                throw e
            }
        }
    }

    private fun stopLocalProxy() {
        proxyProcess?.destroy()
        proxyProcess = null
        Log.d(TAG, "Local proxy stopped")
    }

    // =========================================================================
    // Packet Forwarding
    // =========================================================================

    /**
     * Reads packets from the TUN file descriptor, forwards them to the local
     * SOCKS5 proxy, and writes response packets back to the TUN interface.
     *
     * In production, this is typically handled by a library like
     * `hev-socks5-tunnel` which bridges TUN <-> SOCKS5 efficiently.
     */
    private fun startPacketForward(tunFd: ParcelFileDescriptor) {
        packetForwardJob = serviceScope.launch {
            try {
                val inputStream = FileInputStream(tunFd.fileDescriptor)
                val outputStream = FileOutputStream(tunFd.fileDescriptor)
                val buffer = ByteArray(VPN_MTU + 64) // Extra space for headers

                Log.d(TAG, "Packet forwarding loop started")

                while (isActive) {
                    val bytesRead = inputStream.read(buffer)
                    if (bytesRead <= 0) continue

                    // TODO: Parse IP packet, determine protocol
                    // TODO: Forward to SOCKS5 proxy at 127.0.0.1:1080
                    // TODO: Write response back to outputStream

                    // Placeholder: In a real implementation, this would:
                    // 1. Parse the IP header from the buffer
                    // 2. Create a SOCKS5 connection to the destination
                    // 3. Relay data between the TUN and the SOCKS5 proxy
                    // 4. Write the response back to the TUN
                }
            } catch (e: Exception) {
                if (e is CancellationException) {
                    Log.d(TAG, "Packet forwarding cancelled")
                } else {
                    Log.e(TAG, "Packet forwarding error: ${e.message}", e)
                }
            }
        }
    }

    // =========================================================================
    // Notification
    // =========================================================================

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            getString(R.string.notification_channel_name),
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = getString(R.string.notification_channel_desc)
            setShowBadge(false)
        }
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        manager.createNotificationChannel(channel)
    }

    private fun buildNotification(): Notification {
        val intent = Intent(this, MainActivity::class.java)
        val pendingIntent = PendingIntent.getActivity(
            this, 0, intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        return Notification.Builder(this, CHANNEL_ID)
            .setContentTitle(getString(R.string.notification_title))
            .setContentText(getString(R.string.notification_text))
            .setSmallIcon(R.drawable.ic_vpn_notification)
            .setContentIntent(pendingIntent)
            .setOngoing(true)
            .build()
    }

    // =========================================================================
    // Helpers
    // =========================================================================

    /**
     * Extracts a native library from the app's assets to the internal files directory.
     */
    private fun extractNativeLibrary(assetName: String): File {
        val outFile = File(filesDir, assetName)
        if (!outFile.exists()) {
            assets.open(assetName).use { input ->
                outFile.outputStream().use { output ->
                    input.copyTo(output)
                }
            }
            // Make executable
            outFile.setExecutable(true, false)
        }
        return outFile
    }
}
