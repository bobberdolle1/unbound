package ru.unbound.app.ui.theme

import android.app.Activity
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.unit.dp
import androidx.core.view.WindowCompat

/**
 * LocalComposition for our unified color palette.
 */
val LocalUnboundColors = staticCompositionLocalOf { UnboundColors.dark() }

/**
 * Shape definitions for the app.
 */
val UnboundShapes = Shapes(
    extraSmall = RoundedCornerShape(4.dp),
    small = RoundedCornerShape(8.dp),
    medium = RoundedCornerShape(12.dp),
    large = RoundedCornerShape(16.dp),
    extraLarge = RoundedCornerShape(28.dp)
)

/**
 * Main theme composable. Applies colors, typography, and shapes based on
 * the selected [AppTheme]. Automatically adjusts the system status bar.
 *
 * @param theme The [AppTheme] to apply.
 * @param darkTheme Whether to force dark (used when theme == DARK).
 * @param dynamicColor Whether to use dynamic colors (future Material You support).
 * @param content The composable content.
 */
@Composable
fun UnboundTheme(
    theme: AppTheme = AppTheme.DARK,
    darkTheme: Boolean = isSystemInDarkTheme(),
    dynamicColor: Boolean = false,
    content: @Composable () -> Unit
) {
    val colors = when (theme) {
        AppTheme.DOODLE -> UnboundColors.doodle()
        AppTheme.DARK -> UnboundColors.dark()
        AppTheme.LIGHT -> UnboundColors.light()
    }

    val isDark = theme == AppTheme.DARK || (theme == AppTheme.DARK && darkTheme)

    val colorScheme = if (isDark) {
        darkColorScheme(
            primary = colors.primary,
            secondary = colors.secondary,
            tertiary = colors.accent,
            background = colors.background,
            surface = colors.surface,
            onPrimary = colors.onPrimary,
            onSecondary = colors.onSecondary,
            onBackground = colors.onBackground,
            onSurface = colors.onSurface,
            error = colors.error
        )
    } else {
        lightColorScheme(
            primary = colors.primary,
            secondary = colors.secondary,
            tertiary = colors.accent,
            background = colors.background,
            surface = colors.surface,
            onPrimary = colors.onPrimary,
            onSecondary = colors.onSecondary,
            onBackground = colors.onBackground,
            onSurface = colors.onSurface,
            error = colors.error
        )
    }

    val view = LocalView.current
    if (!view.isInEditMode) {
        SideEffect {
            val window = (view.context as Activity).window
            window.statusBarColor = colors.background.toArgb()
            WindowCompat.getInsetsController(window, view).isAppearanceLightStatusBars =
                theme == AppTheme.LIGHT
        }
    }

    CompositionLocalProvider(LocalUnboundColors provides colors) {
        MaterialTheme(
            colorScheme = colorScheme,
            typography = UnboundTypography,
            shapes = UnboundShapes,
            content = content
        )
    }
}

/**
 * Access the current [UnboundColors] from within the [UnboundTheme].
 */
object UnboundTheme {
    val colors: UnboundColors
        @Composable
        get() = LocalUnboundColors.current
}
