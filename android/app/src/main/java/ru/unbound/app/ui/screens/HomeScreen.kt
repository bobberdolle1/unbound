package ru.unbound.app.ui.screens

import android.content.Intent
import android.net.VpnService
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PowerSettingsNew
import androidx.compose.material.icons.filled.Shield
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.scale
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import ru.unbound.app.R
import ru.unbound.app.data.SettingsManager
import ru.unbound.app.ui.theme.UnboundTheme
import ru.unbound.app.vpn.UnboundVpnService

/**
 * Home screen with VPN connect/disconnect button and status display.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen() {
    val context = LocalContext.current
    val settingsManager = remember { SettingsManager(context) }
    var isConnected by remember { mutableStateOf(false) }
    var isConnecting by remember { mutableStateOf(false) }

    // VPN permission launcher
    val vpnPermissionLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == android.app.Activity.RESULT_OK) {
            // Permission granted, start VPN
            isConnecting = true
            val intent = Intent(context, UnboundVpnService::class.java).apply {
                action = UnboundVpnService.ACTION_CONNECT
            }
            context.startService(intent)
        }
    }

    // Collect VPN state
    LaunchedEffect(Unit) {
        settingsManager.vpnConnectedFlow.collect { connected ->
            isConnected = connected
            isConnecting = false
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = stringResource(R.string.app_name),
                        style = MaterialTheme.typography.headlineMedium,
                        fontWeight = FontWeight.Bold
                    )
                },
                actions = {
                    Icon(
                        Icons.Default.Shield,
                        contentDescription = null,
                        tint = if (isConnected) UnboundTheme.colors.success else UnboundTheme.colors.onBackground
                    )
                    Spacer(modifier = Modifier.width(16.dp))
                }
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .background(UnboundTheme.colors.background)
                .padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            // Animated status indicator
            val scale by animateFloatAsState(
                targetValue = if (isConnected) 1.1f else 1.0f,
                label = "status_scale"
            )

            // Status circle
            Box(
                modifier = Modifier
                    .size(180.dp)
                    .scale(scale)
                    .background(
                        brush = Brush.radialGradient(
                            colors = if (isConnected) {
                                listOf(UnboundTheme.colors.success, UnboundTheme.colors.primary)
                            } else if (isConnecting) {
                                listOf(UnboundTheme.colors.warning, UnboundTheme.colors.accent)
                            } else {
                                listOf(UnboundTheme.colors.border, UnboundTheme.colors.surface)
                            }
                        ),
                        shape = CircleShape
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    Icons.Default.PowerSettingsNew,
                    contentDescription = null,
                    modifier = Modifier.size(64.dp),
                    tint = if (isConnected) UnboundTheme.colors.onPrimary else UnboundTheme.colors.onSurface
                )
            }

            Spacer(modifier = Modifier.height(32.dp))

            // Status text
            Text(
                text = when {
                    isConnected -> stringResource(R.string.status_connected)
                    isConnecting -> stringResource(R.string.status_connecting)
                    else -> stringResource(R.string.status_disconnected)
                },
                style = MaterialTheme.typography.headlineSmall,
                color = UnboundTheme.colors.onBackground,
                fontWeight = FontWeight.SemiBold
            )

            Spacer(modifier = Modifier.height(48.dp))

            // Connect/Disconnect button
            Button(
                onClick = {
                    if (isConnected) {
                        // Disconnect
                        val intent = Intent(context, UnboundVpnService::class.java).apply {
                            action = UnboundVpnService.ACTION_DISCONNECT
                        }
                        context.startService(intent)
                    } else {
                        // Request VPN permission
                        val intent = VpnService.prepare(context)
                        if (intent != null) {
                            vpnPermissionLauncher.launch(intent)
                        } else {
                            // Already has permission
                            isConnecting = true
                            val startIntent = Intent(context, UnboundVpnService::class.java).apply {
                                action = UnboundVpnService.ACTION_CONNECT
                            }
                            context.startService(startIntent)
                        }
                    }
                },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(56.dp),
                colors = ButtonDefaults.buttonColors(
                    containerColor = if (isConnected) UnboundTheme.colors.error else UnboundTheme.colors.primary
                ),
                shape = MaterialTheme.shapes.medium
            ) {
                Text(
                    text = if (isConnected) stringResource(R.string.disconnect) else stringResource(R.string.connect),
                    style = MaterialTheme.typography.titleLarge,
                    color = if (isConnected) UnboundTheme.colors.onPrimary else UnboundTheme.colors.onPrimary
                )
            }
        }
    }
}
