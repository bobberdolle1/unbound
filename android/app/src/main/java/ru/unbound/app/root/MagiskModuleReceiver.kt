package ru.unbound.app.root

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log
import ru.unbound.app.data.SettingsManager

/**
 * BroadcastReceiver for communication with the Magisk/KernelSU root module.
 *
 * The root module can send broadcasts to control the app, and the app can
 * send broadcasts to control the root module.
 *
 * Actions:
 * - ru.unbound.MODULE_STATUS: Module reports its status
 * - ru.unbound.MODULE_CONTROL: App sends control commands to the module
 */
class MagiskModuleReceiver : BroadcastReceiver() {

    companion object {
        private const val TAG = "MagiskModuleReceiver"
        const val ACTION_MODULE_STATUS = "ru.unbound.MODULE_STATUS"
        const val ACTION_MODULE_CONTROL = "ru.unbound.MODULE_CONTROL"

        // Extra keys
        const val EXTRA_COMMAND = "command"
        const val EXTRA_STATUS = "status"

        // Commands
        const val CMD_ENABLE = "enable"
        const val CMD_DISABLE = "disable"
        const val CMD_STATUS = "status"
    }

    override fun onReceive(context: Context, intent: Intent) {
        when (intent.action) {
            ACTION_MODULE_STATUS -> {
                val status = intent.getStringExtra(EXTRA_STATUS)
                Log.d(TAG, "Module status: $status")

                // Update app settings based on module status
                val settingsManager = SettingsManager(context)
                kotlinx.coroutines.CoroutineScope(kotlinx.coroutines.Dispatchers.IO).launch {
                    settingsManager.setRootModuleEnabled(status == "active")
                }
            }

            ACTION_MODULE_CONTROL -> {
                val command = intent.getStringExtra(EXTRA_COMMAND)
                Log.d(TAG, "Received command: $command")
                // Handle incoming commands from the module
                // (e.g., module requests config, status, etc.)
            }
        }
    }

    /**
     * Sends a control command to the Magisk module via broadcast.
     */
    fun sendCommand(context: Context, command: String) {
        val intent = Intent(ACTION_MODULE_CONTROL).apply {
            putExtra(EXTRA_COMMAND, command)
            setPackage("ru.unbound.module") // Module's package name
        }
        context.sendBroadcast(intent)
        Log.d(TAG, "Sent command: $command")
    }
}
