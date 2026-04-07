package ru.unbound.app.ui.theme

import androidx.compose.ui.graphics.Color

// ============================================================================
// Theme Enumeration
// ============================================================================

enum class AppTheme(val displayName: String) {
    DOODLE("Doodle Jump"),
    DARK("Современная тёмная"),
    LIGHT("Современная светлая");

    companion object {
        fun fromString(value: String): AppTheme =
            entries.find { it.name == value } ?: DARK
    }
}

// ============================================================================
// Color Definitions — Doodle Jump Minimalism
// ============================================================================

object DoodleColorPalette {
    val Background = Color(0xFFF5F0E1)       // Warm cream
    val Surface = Color(0xFFFFFEF2)           // Near-white cream
    val Primary = Color(0xFF6ABF69)           // Playful green
    val PrimaryVariant = Color(0xFF4A9F48)
    val Secondary = Color(0xFFFFD23F)         // Bright yellow
    val SecondaryVariant = Color(0xFFE5BC38)
    val Accent = Color(0xFFEA5E5E)            // Fun red
    val OnBackground = Color(0xFF2D2D2D)      // Dark brown-ish
    val OnSurface = Color(0xFF333333)
    val OnPrimary = Color(0xFFFFFFFF)
    val OnSecondary = Color(0xFF2D2D2D)
    val Border = Color(0xFFD4C9A8)
    val CardBackground = Color(0xFFFFF8E7)
    val Success = Color(0xFF4CAF50)
    val Error = Color(0xFFF44336)
    val Warning = Color(0xFFFF9800)
}

// ============================================================================
// Color Definitions — Modern Dark
// ============================================================================

object DarkColorPalette {
    val Background = Color(0xFF000000)         // Pure black (AMOLED)
    val Surface = Color(0xFF121212)            // Elevated surface
    val Primary = Color(0xFF82AAFF)            // Soft blue
    val PrimaryVariant = Color(0xFF5E8DE8)
    val Secondary = Color(0xFFC792EA)          // Purple accent
    val SecondaryVariant = Color(0xFFA876D6)
    val Accent = Color(0xFFF78C6C)             // Coral
    val OnBackground = Color(0xFFE0E0E0)
    val OnSurface = Color(0xFFE8E8E8)
    val OnPrimary = Color(0xFF000000)
    val OnSecondary = Color(0xFF000000)
    val Border = Color(0xFF2A2A2A)
    val CardBackground = Color(0xFF1A1A1A)
    val Success = Color(0xFF4CAF50)
    val Error = Color(0xFFEF5350)
    val Warning = Color(0xFFFFA726)
}

// ============================================================================
// Color Definitions — Modern Light
// ============================================================================

object LightColorPalette {
    val Background = Color(0xFFF8F9FA)         // Clean grey-white
    val Surface = Color(0xFFFFFFFF)            // Pure white
    val Primary = Color(0xFF1A73E8)            // Google blue
    val PrimaryVariant = Color(0xFF1557B0)
    val Secondary = Color(0xFF5F6368)          // Neutral grey
    val SecondaryVariant = Color(0xFF494C51)
    val Accent = Color(0xFFE8710A)             // Orange accent
    val OnBackground = Color(0xFF202124)
    val OnSurface = Color(0xFF202124)
    val OnPrimary = Color(0xFFFFFFFF)
    val OnSecondary = Color(0xFFFFFFFF)
    val Border = Color(0xFFDADCE0)
    val CardBackground = Color(0xFFFFFFFF)
    val Success = Color(0xFF0D904F)
    val Error = Color(0xFFD93025)
    val Warning = Color(0xFFF9AB00)
}

// ============================================================================
// Unified Color Data Class
// ============================================================================

data class UnboundColors(
    val background: Color,
    val surface: Color,
    val primary: Color,
    val primaryVariant: Color,
    val secondary: Color,
    val secondaryVariant: Color,
    val accent: Color,
    val onBackground: Color,
    val onSurface: Color,
    val onPrimary: Color,
    val onSecondary: Color,
    val border: Color,
    val cardBackground: Color,
    val success: Color,
    val error: Color,
    val warning: Color
) {
    companion object {
        fun doodle() = UnboundColors(
            background = DoodleColorPalette.Background,
            surface = DoodleColorPalette.Surface,
            primary = DoodleColorPalette.Primary,
            primaryVariant = DoodleColorPalette.PrimaryVariant,
            secondary = DoodleColorPalette.Secondary,
            secondaryVariant = DoodleColorPalette.SecondaryVariant,
            accent = DoodleColorPalette.Accent,
            onBackground = DoodleColorPalette.OnBackground,
            onSurface = DoodleColorPalette.OnSurface,
            onPrimary = DoodleColorPalette.OnPrimary,
            onSecondary = DoodleColorPalette.OnSecondary,
            border = DoodleColorPalette.Border,
            cardBackground = DoodleColorPalette.CardBackground,
            success = DoodleColorPalette.Success,
            error = DoodleColorPalette.Error,
            warning = DoodleColorPalette.Warning
        )

        fun dark() = UnboundColors(
            background = DarkColorPalette.Background,
            surface = DarkColorPalette.Surface,
            primary = DarkColorPalette.Primary,
            primaryVariant = DarkColorPalette.PrimaryVariant,
            secondary = DarkColorPalette.Secondary,
            secondaryVariant = DarkColorPalette.SecondaryVariant,
            accent = DarkColorPalette.Accent,
            onBackground = DarkColorPalette.OnBackground,
            onSurface = DarkColorPalette.OnSurface,
            onPrimary = DarkColorPalette.OnPrimary,
            onSecondary = DarkColorPalette.OnSecondary,
            border = DarkColorPalette.Border,
            cardBackground = DarkColorPalette.CardBackground,
            success = DarkColorPalette.Success,
            error = DarkColorPalette.Error,
            warning = DarkColorPalette.Warning
        )

        fun light() = UnboundColors(
            background = LightColorPalette.Background,
            surface = LightColorPalette.Surface,
            primary = LightColorPalette.Primary,
            primaryVariant = LightColorPalette.PrimaryVariant,
            secondary = LightColorPalette.Secondary,
            secondaryVariant = LightColorPalette.SecondaryVariant,
            accent = LightColorPalette.Accent,
            onBackground = LightColorPalette.OnBackground,
            onSurface = LightColorPalette.OnSurface,
            onPrimary = LightColorPalette.OnPrimary,
            onSecondary = LightColorPalette.OnSecondary,
            border = LightColorPalette.Border,
            cardBackground = LightColorPalette.CardBackground,
            success = LightColorPalette.Success,
            error = LightColorPalette.Error,
            warning = LightColorPalette.Warning
        )
    }
}
