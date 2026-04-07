package ru.unbound.app.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
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
 * Autostart screen — configure rules for automatic VPN activation.
 * - Boot completed
 * - Specific Wi-Fi SSIDs
 * - Specific apps (via UsageStatsManager)
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AutostartScreen() {
    val context = androidx.compose.ui.platform.LocalContext.current
    val settingsManager = remember { SettingsManager(context) }
    val appDataManager = remember { AppDataManager(context) }

    var settings by remember { mutableStateOf<SettingsManager.AppSettings?>(null) }
    var trustedSsids by remember { mutableStateOf<Set<String>>(emptySet()) }
    var showAddSsidDialog by remember { mutableStateOf(false) }
    var newSsid by remember { mutableStateOf("") }

    // Collect settings
    LaunchedEffect(Unit) {
        settingsManager.settingsFlow.collect { appSettings ->
            settings = appSettings
        }
        appDataManager.trustedWifiSsidsFlow.collect { ssids ->
            trustedSsids = ssids
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = stringResource(R.string.autostart_title),
                        style = MaterialTheme.typography.headlineMedium,
                        fontWeight = FontWeight.Bold
                    )
                }
            )
        }
    ) { padding ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .background(UnboundTheme.colors.background)
                .padding(padding)
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            // Boot trigger
            item {
                AutostartRuleCard(
                    icon = Icons.Default.PowerSettingsNew,
                    title = stringResource(R.string.autostart_boot),
                    subtitle = stringResource(R.string.autostart_boot_desc),
                    enabled = settings?.autostartBoot ?: false,
                    onToggle = { enabled ->
                        settingsManager.setAutostartBoot(enabled)
                    }
                )
            }

            // WiFi trigger
            item {
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.small
                ) {
                    Column(modifier = Modifier.padding(16.dp)) {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.SpaceBetween
                        ) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Icon(Icons.Default.Wifi, contentDescription = null, tint = UnboundTheme.colors.primary)
                                Spacer(modifier = Modifier.width(16.dp))
                                Column {
                                    Text(
                                        text = stringResource(R.string.autostart_wifi),
                                        style = MaterialTheme.typography.titleMedium,
                                        fontWeight = FontWeight.Medium
                                    )
                                    Text(
                                        text = stringResource(R.string.autostart_wifi_desc),
                                        style = MaterialTheme.typography.bodySmall,
                                        color = UnboundTheme.colors.onSurface.copy(alpha = 0.6f)
                                    )
                                }
                            }
                            Switch(
                                checked = settings?.autostartWifi ?: false,
                                onCheckedChange = { enabled ->
                                    settingsManager.setAutostartWifi(enabled)
                                }
                            )
                        }

                        // Trusted SSID list
                        if (trustedSsids.isNotEmpty()) {
                            Spacer(modifier = Modifier.height(12.dp))
                            Divider(color = UnboundTheme.colors.border)
                            Spacer(modifier = Modifier.height(8.dp))

                            trustedSsids.forEach { ssid ->
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .padding(vertical = 4.dp),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Icon(Icons.Default.Wifi, contentDescription = null, tint = UnboundTheme.colors.secondary, modifier = Modifier.size(20.dp))
                                    Spacer(modifier = Modifier.width(8.dp))
                                    Text(ssid, modifier = Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium)
                                    IconButton(onClick = {
                                        appDataManager.removeTrustedWifiSsid(ssid)
                                    }) {
                                        Icon(Icons.Default.Delete, contentDescription = "Удалить", tint = UnboundTheme.colors.error)
                                    }
                                }
                            }
                        }

                        // Add SSID button
                        Spacer(modifier = Modifier.height(8.dp))
                        OutlinedButton(
                            onClick = { showAddSsidDialog = true },
                            modifier = Modifier.fillMaxWidth(),
                            shape = MaterialTheme.shapes.extraSmall
                        ) {
                            Icon(Icons.Default.Add, contentDescription = null)
                            Spacer(modifier = Modifier.width(4.dp))
                            Text(stringResource(R.string.autostart_add_wifi))
                        }
                    }
                }
            }

            // App trigger
            item {
                AutostartRuleCard(
                    icon = Icons.Default.Apps,
                    title = stringResource(R.string.autostart_apps),
                    subtitle = stringResource(R.string.autostart_apps_desc),
                    enabled = settings?.autostartApps ?: false,
                    onToggle = { enabled ->
                        settingsManager.setAutostartApps(enabled)
                    }
                )
            }
        }
    }

    // Add SSID Dialog
    if (showAddSsidDialog) {
        AlertDialog(
            onDismissRequest = { showAddSsidDialog = false },
            title = { Text(stringResource(R.string.autostart_add_wifi)) },
            text = {
                OutlinedTextField(
                    value = newSsid,
                    onValueChange = { newSsid = it },
                    label = { Text("Wi-Fi SSID") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
            },
            confirmButton = {
                Button(
                    onClick = {
                        if (newSsid.isNotBlank()) {
                            appDataManager.addTrustedWifiSsid(newSsid)
                            newSsid = ""
                            showAddSsidDialog = false
                        }
                    }
                ) {
                    Text(stringResource(R.string.save))
                }
            },
            dismissButton = {
                TextButton(onClick = { showAddSsidDialog = false }) {
                    Text(stringResource(R.string.cancel))
                }
            }
        )
    }
}

@Composable
private fun AutostartRuleCard(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    title: String,
    subtitle: String,
    enabled: Boolean,
    onToggle: (Boolean) -> Unit
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.small
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(icon, contentDescription = null, tint = UnboundTheme.colors.primary)
                Spacer(modifier = Modifier.width(16.dp))
                Column {
                    Text(
                        text = title,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Medium
                    )
                    Text(
                        text = subtitle,
                        style = MaterialTheme.typography.bodySmall,
                        color = UnboundTheme.colors.onSurface.copy(alpha = 0.6f)
                    )
                }
            }
            Switch(
                checked = enabled,
                onCheckedChange = onToggle
            )
        }
    }
}
