package ru.unbound.app.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
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
import ru.unbound.app.data.SettingsManager
import ru.unbound.app.ui.theme.AppTheme
import ru.unbound.app.ui.theme.UnboundTheme

/**
 * Settings screen with theme selector, proxy config, and DNS settings.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen() {
    val context = androidx.compose.ui.platform.LocalContext.current
    val settingsManager = remember { SettingsManager(context) }
    var settings by remember { mutableStateOf<SettingsManager.AppSettings?>(null) }
    var showThemeDialog by remember { mutableStateOf(false) }
    var proxyHost by remember { mutableStateOf("127.0.0.1") }
    var proxyPort by remember { mutableStateOf("1080") }
    var dnsServer by remember { mutableStateOf("") }

    // Collect settings
    LaunchedEffect(Unit) {
        settingsManager.settingsFlow.collect { appSettings ->
            settings = appSettings
            proxyHost = appSettings.proxyHost
            proxyPort = appSettings.proxyPort.toString()
            dnsServer = appSettings.dnsServer
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = stringResource(R.string.nav_settings),
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
                .verticalScroll(rememberScrollState())
                .padding(padding)
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            // Theme Section
            SettingsSectionTitle(stringResource(R.string.settings_theme))

            SettingsCard(
                icon = Icons.Default.Palette,
                title = settings?.theme?.displayName ?: AppTheme.DARK.displayName,
                subtitle = "Нажмите для выбора темы",
                onClick = { showThemeDialog = true }
            )

            // Proxy Section
            SettingsSectionTitle(stringResource(R.string.settings_proxy))

            OutlinedTextField(
                value = proxyHost,
                onValueChange = { proxyHost = it },
                label = { Text(stringResource(R.string.settings_proxy_host)) },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                shape = MaterialTheme.shapes.small
            )

            OutlinedTextField(
                value = proxyPort,
                onValueChange = { proxyPort = it },
                label = { Text(stringResource(R.string.settings_proxy_port)) },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                shape = MaterialTheme.shapes.small
            )

            Button(
                onClick = {
                    val port = proxyPort.toIntOrNull() ?: 1080
                    settingsManager.setProxySettings(proxyHost, port)
                },
                modifier = Modifier.fillMaxWidth(),
                shape = MaterialTheme.shapes.small
            ) {
                Text(stringResource(R.string.save))
            }

            // DNS Section
            SettingsSectionTitle(stringResource(R.string.settings_dns))

            OutlinedTextField(
                value = dnsServer,
                onValueChange = { dnsServer = it },
                label = { Text(stringResource(R.string.settings_dns)) },
                placeholder = { Text(stringResource(R.string.settings_dns_auto)) },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                shape = MaterialTheme.shapes.small
            )

            Button(
                onClick = {
                    settingsManager.setDnsServer(dnsServer)
                },
                modifier = Modifier.fillMaxWidth(),
                shape = MaterialTheme.shapes.small
            ) {
                Text(stringResource(R.string.save))
            }

            // Root Module Section (if available)
            SettingsSectionTitle(stringResource(R.string.module_title))

            SwitchSettingCard(
                icon = Icons.Default.Security,
                title = stringResource(R.string.module_enable),
                subtitle = if (settings?.rootModuleEnabled == true)
                    stringResource(R.string.module_installed)
                else
                    stringResource(R.string.module_not_installed),
                checked = settings?.rootModuleEnabled ?: false,
                onCheckedChange = { enabled ->
                    settingsManager.setRootModuleEnabled(enabled)
                }
            )
        }
    }

    // Theme Selection Dialog
    if (showThemeDialog) {
        AlertDialog(
            onDismissRequest = { showThemeDialog = false },
            title = { Text(stringResource(R.string.settings_theme)) },
            text = {
                Column {
                    AppTheme.entries.forEach { theme ->
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable {
                                    settingsManager.setTheme(theme)
                                    showThemeDialog = false
                                }
                                .padding(vertical = 12.dp),
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            RadioButton(
                                selected = settings?.theme == theme,
                                onClick = {
                                    settingsManager.setTheme(theme)
                                    showThemeDialog = false
                                }
                            )
                            Spacer(modifier = Modifier.width(12.dp))
                            Text(theme.displayName)
                        }
                    }
                }
            },
            confirmButton = {
                TextButton(onClick = { showThemeDialog = false }) {
                    Text(stringResource(R.string.cancel))
                }
            }
        )
    }
}

@Composable
private fun SettingsSectionTitle(title: String) {
    Text(
        text = title,
        style = MaterialTheme.typography.titleMedium,
        fontWeight = FontWeight.SemiBold,
        color = UnboundTheme.colors.primary,
        modifier = Modifier.padding(top = 8.dp, bottom = 4.dp)
    )
}

@Composable
private fun SettingsCard(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    title: String,
    subtitle: String,
    onClick: () -> Unit
) {
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = MaterialTheme.shapes.small
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Icon(icon, contentDescription = null, tint = UnboundTheme.colors.primary)
            Spacer(modifier = Modifier.width(16.dp))
            Column {
                Text(title, style = MaterialTheme.typography.titleMedium)
                Text(subtitle, style = MaterialTheme.typography.bodySmall, color = UnboundTheme.colors.onSurface.copy(alpha = 0.6f))
            }
        }
    }
}

@Composable
private fun SwitchSettingCard(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    title: String,
    subtitle: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit
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
                    Text(title, style = MaterialTheme.typography.titleMedium)
                    Text(subtitle, style = MaterialTheme.typography.bodySmall, color = UnboundTheme.colors.onSurface.copy(alpha = 0.6f))
                }
            }
            Switch(
                checked = checked,
                onCheckedChange = onCheckedChange
            )
        }
    }
}
