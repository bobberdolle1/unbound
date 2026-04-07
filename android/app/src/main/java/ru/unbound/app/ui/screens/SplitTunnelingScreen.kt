package ru.unbound.app.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import ru.unbound.app.R
import ru.unbound.app.data.AppDataManager
import ru.unbound.app.data.SettingsManager
import ru.unbound.app.ui.theme.UnboundTheme

/**
 * Data class representing an installed app for split tunneling.
 */
data class AppInfo(
    val packageName: String,
    val appName: String,
    val isSystemApp: Boolean,
    val icon: androidx.compose.ui.graphics.vector.ImageVector = Icons.Default.Android
)

/**
 * Split Tunneling screen — allows users to include/exclude specific apps.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SplitTunnelingScreen() {
    val context = androidx.compose.ui.platform.LocalContext.current
    val settingsManager = remember { SettingsManager(context) }
    val appDataManager = remember { AppDataManager(context) }

    var splitMode by remember { mutableIntStateOf(0) }
    var searchQuery by remember { mutableStateOf("") }
    var showSystemApps by remember { mutableStateOf(false) }
    var disallowedApps by remember { mutableStateOf<Set<String>>(emptySet()) }

    // Collect settings
    LaunchedEffect(Unit) {
        settingsManager.splitTunnelModeFlow.collect { mode ->
            splitMode = mode
        }
        appDataManager.disallowedAppsFlow.collect { apps ->
            disallowedApps = apps
        }
    }

    // Mock installed apps (in production, use PackageManager to get real list)
    val allApps = remember {
        listOf(
            AppInfo("com.google.chrome", "Chrome", false),
            AppInfo("com.telegram", "Telegram", false),
            AppInfo("com.whatsapp", "WhatsApp", false),
            AppInfo("com.android.system", "System UI", true),
            AppInfo("com.google.play", "Google Play", true),
            AppInfo("com.youtube", "YouTube", false),
            AppInfo("com.instagram", "Instagram", false),
            AppInfo("com.twitter", "Twitter", false),
        )
    }

    val filteredApps = allApps
        .filter { app -> app.appName.contains(searchQuery, ignoreCase = true) }
        .filter { app -> showSystemApps || !app.isSystemApp }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = stringResource(R.string.split_tunnel_title),
                        style = MaterialTheme.typography.headlineMedium,
                        fontWeight = FontWeight.Bold
                    )
                }
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .background(UnboundTheme.colors.background)
                .padding(padding)
        ) {
            // Mode selector
            Card(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp),
                shape = MaterialTheme.shapes.small
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = "Режим раздельного туннелирования",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold
                    )
                    Spacer(modifier = Modifier.height(8.dp))

                    listOf(
                        0 to stringResource(R.string.split_tunnel_mode_all),
                        1 to stringResource(R.string.split_tunnel_mode_exclude),
                        2 to stringResource(R.string.split_tunnel_mode_include)
                    ).forEach { (mode, label) ->
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable { settingsManager.setSplitTunnelMode(mode) }
                                .padding(vertical = 4.dp),
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            RadioButton(
                                selected = splitMode == mode,
                                onClick = { settingsManager.setSplitTunnelMode(mode) }
                            )
                            Spacer(modifier = Modifier.width(8.dp))
                            Text(label)
                        }
                    }
                }
            }

            // Search bar
            OutlinedTextField(
                value = searchQuery,
                onValueChange = { searchQuery = it },
                placeholder = { Text(stringResource(R.string.split_tunnel_search)) },
                leadingIcon = {
                    Icon(Icons.Default.Search, contentDescription = null)
                },
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp),
                singleLine = true,
                shape = MaterialTheme.shapes.small
            )

            // Show system apps toggle
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clickable { showSystemApps = !showSystemApps }
                    .padding(16.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Text(
                    text = stringResource(R.string.split_tunnel_system),
                    style = MaterialTheme.typography.bodyMedium
                )
                Switch(
                    checked = showSystemApps,
                    onCheckedChange = { showSystemApps = it }
                )
            }

            Divider(color = UnboundTheme.colors.border)

            // App list
            LazyColumn(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = 16.dp),
                contentPadding = PaddingValues(vertical = 8.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                items(filteredApps) { app ->
                    AppListItem(
                        app = app,
                        isDisallowed = app.packageName in disallowedApps,
                        onToggle = { checked ->
                            if (checked) {
                                appDataManager.addDisallowedApp(app.packageName)
                            } else {
                                appDataManager.removeDisallowedApp(app.packageName)
                            }
                        }
                    )
                }
            }
        }
    }
}

@Composable
private fun AppListItem(
    app: AppInfo,
    isDisallowed: Boolean,
    onToggle: (Boolean) -> Unit
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.extraSmall
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Icon(
                app.icon,
                contentDescription = null,
                tint = UnboundTheme.colors.onSurface,
                modifier = Modifier.size(32.dp)
            )
            Spacer(modifier = Modifier.width(12.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = app.appName,
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = app.packageName,
                    style = MaterialTheme.typography.bodySmall,
                    color = UnboundTheme.colors.onSurface.copy(alpha = 0.5f)
                )
            }
            Checkbox(
                checked = isDisallowed,
                onCheckedChange = onToggle
            )
        }
    }
}
