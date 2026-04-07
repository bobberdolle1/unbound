package ru.unbound.app.ui.screens

import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.List
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Timer
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import ru.unbound.app.R

/**
 * Navigation routes
 */
object Routes {
    const val HOME = "home"
    const val SETTINGS = "settings"
    const val SPLIT_TUNNELING = "split_tunneling"
    const val AUTOSTART = "autostart"
}

/**
 * Navigation items for the bottom bar
 */
data class NavItem(
    val route: String,
    val icon: androidx.compose.ui.graphics.vector.ImageVector,
    val labelResId: Int
)

val bottomNavItems = listOf(
    NavItem(Routes.HOME, Icons.Default.Home, R.string.nav_home),
    NavItem(Routes.SETTINGS, Icons.Default.Settings, R.string.nav_settings),
    NavItem(Routes.SPLIT_TUNNELING, Icons.Default.List, R.string.nav_split_tunneling),
    NavItem(Routes.AUTOSTART, Icons.Default.Timer, R.string.nav_autostart)
)

/**
 * Main screen with bottom navigation and NavHost.
 */
@Composable
fun MainScreen() {
    val navController = rememberNavController()
    val navBackStackEntry by navController.currentBackStackEntryAsState()
    val currentRoute = navBackStackEntry?.destination?.route

    Scaffold(
        bottomBar = {
            NavigationBar {
                bottomNavItems.forEach { item ->
                    NavigationBarItem(
                        icon = { Icon(item.icon, contentDescription = null) },
                        label = { Text(stringResource(item.labelResId)) },
                        selected = currentRoute == item.route,
                        onClick = {
                            navController.navigate(item.route) {
                                popUpTo(navController.graph.startDestinationId) {
                                    saveState = true
                                }
                                launchSingleTop = true
                                restoreState = true
                            }
                        }
                    )
                }
            }
        }
    ) { padding ->
        NavHost(
            navController = navController,
            startDestination = Routes.HOME,
            modifier = Modifier.padding(padding)
        ) {
            composable(Routes.HOME) { HomeScreen() }
            composable(Routes.SETTINGS) { SettingsScreen() }
            composable(Routes.SPLIT_TUNNELING) { SplitTunnelingScreen() }
            composable(Routes.AUTOSTART) { AutostartScreen() }
        }
    }
}
