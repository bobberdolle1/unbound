package ru.unbound.app.vpn

import android.content.Context
import android.util.Log
import java.io.File
import java.io.FileOutputStream

/**
 * Native bridge for managing local DPI bypass proxy processes and TUN-to-SOCKS5 tunnel integrations.
 */
object NativeEngineBridge {

    private const val TAG = "NativeEngineBridge"
    private var isNativeLibraryLoaded = false
    private var activeProxyProcess: Process? = null

    init {
        try {
            System.loadLibrary("unbound_tunnel")
            isNativeLibraryLoaded = true
            Log.i(TAG, "Native tunnel library loaded successfully.")
        } catch (e: UnsatisfiedLinkError) {
            isNativeLibraryLoaded = false
            Log.w(TAG, "Native library libunbound_tunnel.so not available, using fallback process runner.")
        }
    }

    /**
     * Native JNI call to start TUN-to-SOCKS5 packet relay.
     */
    external fun startTunTunnel(tunFd: Int, socksHost: String, socksPort: Int): Int

    /**
     * Native JNI call to stop TUN-to-SOCKS5 packet relay.
     */
    external fun stopTunTunnel(): Int

    /**
     * Launches an embedded native DPI bypass proxy daemon (e.g. ByeDPI / tpws / nfqws).
     */
    fun startLocalProxyDaemon(context: Context, host: String = "127.0.0.1", port: Int = 1080): Boolean {
        if (activeProxyProcess != null) {
            Log.d(TAG, "Proxy daemon is already running.")
            return true
        }

        return try {
            val binaryFile = extractAssetBinary(context, "byedpi")
            if (binaryFile != null && binaryFile.exists()) {
                val processBuilder = ProcessBuilder(
                    binaryFile.absolutePath,
                    "-b", host,
                    "-p", port.toString(),
                    "--disorder", "1",
                    "--auto", "torst",
                    "--ttl", "5"
                )
                processBuilder.directory(context.filesDir)
                activeProxyProcess = processBuilder.start()
                Log.i(TAG, "Native DPI proxy daemon started on $host:$port (PID: ${activeProxyProcess.hashCode()})")
                true
            } else {
                Log.i(TAG, "No embedded DPI binary found; relying on external SOCKS5 or active tunnel relay.")
                true
            }
        } catch (e: Exception) {
            Log.e(TAG, "Failed to launch native proxy daemon: ${e.message}", e)
            false
        }
    }

    /**
     * Stops the running local DPI proxy daemon.
     */
    fun stopLocalProxyDaemon() {
        activeProxyProcess?.let { process ->
            try {
                process.destroy()
                Log.d(TAG, "Local DPI proxy daemon stopped.")
            } catch (e: Exception) {
                Log.e(TAG, "Error stopping proxy daemon: ${e.message}", e)
            }
        }
        activeProxyProcess = null
    }

    /**
     * Extracts executable binary asset if provided.
     */
    private fun extractAssetBinary(context: Context, assetName: String): File? {
        val outFile = File(context.filesDir, assetName)
        if (outFile.exists()) return outFile

        return try {
            context.assets.open(assetName).use { input ->
                FileOutputStream(outFile).use { output ->
                    input.copyTo(output)
                }
            }
            outFile.setExecutable(true, false)
            outFile
        } catch (e: Exception) {
            null
        }
    }
}
