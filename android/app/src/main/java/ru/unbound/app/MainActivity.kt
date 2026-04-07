package ru.unbound.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import ru.unbound.app.data.SettingsManager
import ru.unbound.app.ui.screens.MainScreen
import ru.unbound.app.ui.theme.UnboundTheme

/**
 * Main entry point of the Unbound application.
 * Hosts the navigation scaffold and applies the selected theme.
 */
class MainActivity : ComponentActivity() {

    private lateinit var settingsManager: SettingsManager

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        settingsManager = SettingsManager(this)

        setContent {
            val theme by settingsManager.themeFlow.collectAsState(initial = ru.unbound.app.ui.theme.AppTheme.DARK)

            UnboundTheme(theme = theme) {
                MainScreen()
            }
        }
    }
}
