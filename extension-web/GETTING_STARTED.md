# Unbound Web - Getting Started Guide

## Quick Start

### 1. Install Dependencies

```bash
cd extension-web
npm install
```

### 2. Development Mode

For Chrome:
```bash
npm run dev:chrome
```

For Firefox:
```bash
npm run dev:firefox
```

### 3. Load Extension in Browser

#### Chrome
1. Open `chrome://extensions/`
2. Enable **Developer mode** (top-right toggle)
3. Click **Load unpacked**
4. Select the `dist/chrome` folder

#### Firefox
1. Open `about:debugging#/runtime/this-firefox`
2. Click **Load Temporary Add-on**
3. Select any file from `dist/firefox`

### 4. Build for Production

```bash
# Build for both browsers
npm run build

# Or build individually
npm run build:chrome
npm run build:firefox
```

The built extensions will be in:
- `dist/chrome/` - Chrome build
- `dist/firefox/` - Firefox build

---

## Using the Extension

### Main Interface

The popup has these components:

1. **CONNECT/DISCONNECT Toggle** - Large circular button to activate/deactivate
2. **Mode Selector** - Choose between:
   - **Companion Mode**: Uses local Unbound Desktop daemon
   - **Standalone Mode**: Uses external proxy server
3. **Theme Switcher** - Toggle between Doodle Jump (light) and Modern Dark themes
4. **Bypass Domains** - Add domains to route through proxy (e.g., `*.youtube.com`)
5. **Proxy Configuration** - (Standalone mode only) Configure HTTPS/SOCKS5 proxy

### Companion Mode Setup

Companion mode requires the Unbound Desktop application to be installed:

#### Windows
1. Ensure Unbound Desktop is installed and running
2. Register the native messaging host:
   ```powershell
   cd extension-web
   .\scripts\register-host.ps1
   ```
3. Update `host_manifest.json` with:
   - Correct path to your host binary
   - Actual extension ID (visible at `chrome://extensions/`)

#### macOS
```bash
mkdir -p ~/Library/Application\ Support/Google/Chrome/NativeMessagingHosts
cp host_manifest.json ~/Library/Application\ Support/Google/Chrome/NativeMessagingHosts/com.unbound.desktop.json
```

#### Linux
```bash
mkdir -p /etc/opt/chrome/native-messaging-hosts
cp host_manifest.json /etc/opt/chrome/native-messaging-hosts/com.unbound.desktop.json
```

### Standalone Mode Setup

1. Switch to **Standalone** mode
2. Click **Edit** in the Proxy Server section
3. Enter your proxy details:
   - Protocol: HTTPS or SOCKS5
   - Host: `proxy.example.com`
   - Port: `8080`
4. Click **Save**
5. Add domains to the bypass list
6. Click the **CONNECT** toggle

The extension will generate a PAC script that routes only specified domains through your proxy.

---

## Architecture Overview

```
┌─────────────────────────────────────────────┐
│           Popup UI (React)                  │
│  - ConnectToggle                            │
│  - ModeSelector                             │
│  - DomainList                               │
│  - ProxyConfigPanel                         │
└──────────────┬──────────────────────────────┘
               │ chrome.runtime.sendMessage
               ▼
┌─────────────────────────────────────────────┐
│      Background Service Worker              │
│  ┌──────────────┐    ┌──────────────────┐  │
│  │ Companion    │    │   Standalone     │  │
│  │   Mode       │    │     Mode         │  │
│  │              │    │                  │  │
│  │ Native       │    │ PAC Script       │  │
│  │ Messaging    │    │ Generator        │  │
│  └──────┬───────┘    └────────┬─────────┘  │
└─────────┼────────────────────┼─────────────┘
          │                    │
          ▼                    ▼
┌─────────────────┐  ┌──────────────────────┐
│ Unbound Desktop │  │ Browser Proxy API    │
│   (Native App)  │  │ (chrome.proxy.*)     │
└─────────────────┘  └──────────────────────┘
```

---

## File Structure

```
extension-web/
├── src/
│   ├── background/              # Service worker code
│   │   ├── index.ts             # Main entry, message routing
│   │   ├── companion.ts         # Native messaging integration
│   │   └── standalone.ts        # PAC script & proxy logic
│   │
│   ├── popup/                   # React popup UI
│   │   ├── main.tsx             # React entry point
│   │   └── App.tsx              # Main application component
│   │
│   ├── components/              # React components
│   │   ├── ConnectToggle.tsx    # Big connect/disconnect button
│   │   ├── ModeSelector.tsx     # Companion/Standalone switch
│   │   ├── DomainList.tsx       # Domain management
│   │   ├── ThemeSwitcher.tsx    # Light/dark theme toggle
│   │   └── ProxyConfigPanel.tsx # Proxy server configuration
│   │
│   ├── utils/                   # Utility functions
│   │   ├── storage.ts           # State persistence
│   │   ├── proxy.ts             # PAC script generation
│   │   └── browser.ts           # Cross-browser helpers
│   │
│   ├── types/                   # TypeScript definitions
│   │   └── index.ts
│   │
│   └── styles/                  # Global styles
│       └── globals.css
│
├── public/
│   └── icons/                   # Extension icons (SVG)
│
├── scripts/                     # Helper scripts
│   ├── register-host.ps1        # Register native host (Windows)
│   ├── unregister-host.ps1      # Unregister native host
│   └── load-extension.ps1       # Quick load guide
│
├── manifest.chrome.ts           # Chrome MV3 manifest
├── manifest.firefox.ts          # Firefox MV3 manifest
├── vite.config.ts               # Vite build config
├── tailwind.config.js           # Tailwind CSS config
├── host_manifest.json           # Native messaging host config
└── package.json
```

---

## Key Features

### 1. Dual Mode Operation

**Companion Mode:**
- Communicates with local Unbound Desktop app
- Full DPI/censorship bypass capabilities
- Requires native messaging host registration

**Standalone Mode:**
- Works without external applications
- Uses browser's proxy API
- Routes specific domains through external proxy
- Generates dynamic PAC scripts

### 2. Dynamic PAC Generation

PAC (Proxy Auto-Configuration) scripts are generated on-the-fly:

```javascript
function FindProxyForURL(url, host) {
  if (dnsDomainIs(host, '.youtube.com') ||
      dnsDomainIs(host, '.discord.com')) {
    return "PROXY proxy.example.com:8080";
  }
  return "DIRECT";
}
```

### 3. Theme Engine

Two themes available:
- **Doodle Jump Minimalism**: Light, warm colors
- **Modern Dark**: Dark blue/red color scheme

Themes are persisted across sessions and applied instantly.

### 4. State Persistence

All settings are stored in `chrome.storage.local`:
- Mode preference
- Connection status
- Theme selection
- Bypass domains list
- Proxy configuration

---

## Native Messaging Protocol

### Extension → Host

```json
// Start bypass
{"command": "start", "domains": ["*.youtube.com"]}

// Stop bypass
{"command": "stop"}

// Query status
{"command": "status"}

// Update domains
{"command": "update_domains", "domains": ["*.youtube.com", "*.discord.com"]}
```

### Host → Extension

```json
// Running successfully
{"status": "running", "version": "1.0.0"}

// Stopped
{"status": "stopped"}

// Error occurred
{"status": "error", "message": "Port already in use"}
```

---

## Development Tips

### Debugging

**Chrome:**
- Service Worker: `chrome://extensions/` → Click "Service Worker" link
- Popup: Right-click extension icon → "Inspect popup"
- Console logs visible in DevTools

**Firefox:**
- Service Worker: `about:debugging#/runtime/this-firefox` → Inspect
- Popup: Click extension icon, right-click → "Inspect"

### Common Issues

**Native Messaging Not Working:**
- Check registry entries are correct
- Verify `host_manifest.json` has correct extension ID
- Ensure Unbound Desktop is running
- Check browser console for errors

**PAC Script Not Applying:**
- Verify domains are in correct format (`*.example.com`)
- Check proxy server is accessible
- Review `chrome://settings/security` for proxy restrictions

**Build Errors:**
```bash
# Clear node_modules and reinstall
rm -rf node_modules
npm install

# Clear Vite cache
rm -rf node_modules/.vite
```

---

## Production Deployment

### Chrome Web Store

1. Build for Chrome: `npm run build:chrome`
2. Zip the `dist/chrome` folder
3. Upload to [Chrome Web Store Developer Dashboard](https://chrome.google.com/webstore/devconsole/)

### Firefox Add-ons

1. Build for Firefox: `npm run build:firefox`
2. Zip the `dist/firefox` folder
3. Upload to [Firefox Add-ons Developer Hub](https://addons.mozilla.org/developers/)

---

## Next Steps

- [ ] Add proxy connectivity testing
- [ ] Implement import/export domain lists
- [ ] Add statistics and usage metrics
- [ ] Create system tray icon for desktop companion
- [ ] Add support for multiple proxy configurations
- [ ] Implement automatic proxy detection

---

## Support

For issues or questions:
- Check the main README.md
- Review the architecture documentation
- Inspect browser console for errors
- Verify native messaging host registration
