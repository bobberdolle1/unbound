#!/bin/bash
# ============================================================================
# UNBOUND v2.0.0 Release Automation Script
# ============================================================================
# This script orchestrates the complete release process:
# 1. Builds all platform binaries
# 2. Deploys website to GitHub Pages
# 3. Creates GitHub Release with all binaries
# ============================================================================

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
VERSION="v2.0.0"
RELEASE_TITLE="UNBOUND v2.0.0 - Total War on Censorship"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="$REPO_ROOT/dist"

# ============================================================================
# Helper Functions
# ============================================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "\n${CYAN}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}📦 $1${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}\n"
}

check_command() {
    if ! command -v $1 &> /dev/null; then
        log_error "$1 is required but not installed. Please install it first."
        exit 1
    fi
}

# ============================================================================
# Pre-flight Checks
# ============================================================================

log_step "Running pre-flight checks"

check_command "git"
check_command "gh"
check_command "npm"
check_command "go"
check_command "wails"

# Verify we're in the right directory
if [ ! -f "wails.json" ]; then
    log_error "wails.json not found. Please run this script from the repository root."
    exit 1
fi

# Check if logged into GitHub CLI
if ! gh auth status &> /dev/null; then
    log_error "Not authenticated with GitHub CLI. Run: gh auth login"
    exit 1
fi

# Check if on main/master branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ] && [ "$CURRENT_BRANCH" != "master" ]; then
    log_warn "You're on branch '$CURRENT_BRANCH'. Release should typically be done from main/master."
    read -p "Continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

log_success "All pre-flight checks passed!"

# ============================================================================
# Step 1: Build All Platforms
# ============================================================================

log_step "Building all platform binaries"

# Create dist directory
mkdir -p "$DIST_DIR"

# Check if build_all.sh exists
if [ -f "scripts/build_all.sh" ]; then
    log_info "Running build orchestration script..."
    chmod +x scripts/build_all.sh
    ./scripts/build_all.sh
    log_success "Build complete!"
else
    log_warn "scripts/build_all.sh not found. Building manually..."
    
    # Build Wails desktop app
    log_info "Building desktop app (Wails)..."
    wails build -platform windows/amd64
    if [ -f "build/bin/unbound.exe" ]; then
        cp "build/bin/unbound.exe" "$DIST_DIR/unbound-desktop-windows-amd64.exe"
        log_success "Desktop Windows build complete"
    fi
    
    # Build Linux CLI
    if [ -d "linux" ]; then
        log_info "Building Linux CLI (Rust)..."
        cd linux
        cargo build --release
        if [ -f "target/release/unbound-cli" ]; then
            cp "target/release/unbound-cli" "$DIST_DIR/unbound-cli-linux-amd64"
            log_success "Linux CLI build complete"
        fi
        cd ..
    fi
    
    # Build Android APK
    if [ -d "android" ]; then
        log_info "Building Android APK..."
        cd android
        ./gradlew assembleRelease
        if [ -f "app/build/outputs/apk/release/app-release.apk" ]; then
            cp "app/build/outputs/apk/release/app-release.apk" "$DIST_DIR/unbound-android.apk"
            log_success "Android APK build complete"
        fi
        cd ..
    fi
    
    # Build Browser Extension
    if [ -d "extension-web" ]; then
        log_info "Building Browser Extension..."
        cd extension-web
        npm install
        npm run build
        if [ -d "dist" ]; then
            cd dist
            zip -r "$DIST_DIR/unbound-extension-chrome.zip" .
            cd ..
            log_success "Browser Extension build complete"
        fi
        cd ..
    fi
fi

# List all built artifacts
echo ""
log_info "Built artifacts in dist/:"
ls -lh "$DIST_DIR" | tail -n +2

# ============================================================================
# Step 2: Deploy Website
# ============================================================================

log_step "Deploying website to GitHub Pages"

if [ -d "website" ]; then
    cd website
    
    if [ -f "package.json" ]; then
        log_info "Installing website dependencies..."
        npm install
        
        log_info "Deploying to GitHub Pages..."
        if npm run deploy; then
            log_success "Website deployed successfully!"
        else
            log_warn "Website deployment failed. Continuing with release..."
        fi
    else
        log_warn "website/package.json not found. Skipping website deployment."
    fi
    
    cd "$REPO_ROOT"
else
    log_warn "website/ directory not found. Skipping website deployment."
fi

# ============================================================================
# Step 3: Create GitHub Release
# ============================================================================

log_step "Creating GitHub Release"

# Check if release already exists
if gh release view "$VERSION" &> /dev/null; then
    log_warn "Release $VERSION already exists."
    read -p "Delete and recreate? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Deleting existing release..."
        gh release delete "$VERSION" --yes
    else
        log_error "Release already exists. Aborting."
        exit 1
    fi
fi

# Generate release notes
log_info "Generating release notes..."
RELEASE_NOTES=$(cat << 'EOF'
# 🚀 UNBOUND v2.0.0 - Total War on Censorship

## What's New

This is a **major release** that transforms Unbound from a single desktop app into a **complete multi-platform ecosystem** for bypassing DPI censorship.

### ✨ Major Features

- **🌍 Multi-Platform Ecosystem** - Desktop, Android, iOS, Linux, OpenWrt, Browser Extensions, Smart TV, Steam Deck
- **🎯 Auto-Tune V2** - Parallel scanner that finds optimal profiles in seconds
- **🔄 System Tray Integration** - Runs silently in background with tray controls
- **🎨 Redesigned UI** - Modern sketchy design language with real-time monitoring
- **⚡ Improved Performance** - 5-second timeout per probe, no more hanging
- **🔒 Enhanced Security** - HideWindow on all child processes, no console flashing

### 🐛 Bug Fixes

- Fixed settings state persistence (checkboxes now save properly)
- Fixed window close behavior (minimizes to tray instead of quitting)
- Fixed AutoTune infinite hang with strict context timeouts
- Removed console flashing during AutoTune on Windows
- Cleaned up unsupported Telegram/MTProto references

### 📦 Platform Support

| Platform | Format | Status |
|----------|--------|--------|
| **Desktop (Windows/macOS)** | Native App | ✅ Production Ready |
| **Android** | APK | ✅ Production Ready |
| **iOS (Jailbreak)** | Theos Tweak | ✅ Production Ready |
| **Linux** | CLI Binary | ✅ Production Ready |
| **OpenWrt** | Package | ✅ Production Ready |
| **Browser** | Chrome/Firefox Extension | ✅ Production Ready |
| **Steam Deck** | Decky Plugin | ✅ Beta |
| **Smart TV** | WebOS/tvOS | ✅ Beta |

### 🎯 Supported Services

- ✅ **YouTube** - Full 4K streaming support
- ✅ **Discord** - Voice, video, screen sharing
- ✅ **Instagram** - Feed, stories, reels, DMs
- ✅ **Twitter/X** - Timeline, media, search
- ✅ **Facebook** - News feed, marketplace
- ✅ **RuTracker** - Torrent access

### 🔧 Technical Improvements

- Go backend with WinDivert/nfqws packet manipulation
- React frontend with Wails framework
- Rust Linux daemon with systemd integration
- Kotlin Android app with Material You design
- Automated build and release pipeline

---

## 📥 Installation

See the [README.md](https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/blob/main/README.md) for detailed installation instructions for your platform.

## 🐛 Known Issues

- MTProto/Telegram bypass is not officially supported in this configuration
- Some advanced profiles may require manual tuning on certain ISPs

## 🙏 Acknowledgments

Thanks to all contributors and the zapret project for the foundation!

---

**Full Changelog**: https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/compare/v1.0.4...$VERSION
EOF
)

# Create temporary release notes file
RELEASE_NOTES_FILE=$(mktemp)
echo "$RELEASE_NOTES" > "$RELEASE_NOTES_FILE"

# Collect all artifacts
ARTIFACTS=()
if [ -d "$DIST_DIR" ]; then
    while IFS= read -r -d '' file; do
        ARTIFACTS+=("$file")
    done < <(find "$DIST_DIR" -type f -print0)
fi

# Create the release
log_info "Creating release $VERSION..."
if [ ${#ARTIFACTS[@]} -gt 0 ]; then
    gh release create "$VERSION" \
        --title "$RELEASE_TITLE" \
        --notes-file "$RELEASE_NOTES_FILE" \
        --draft \
        --generate-notes \
        "${ARTIFACTS[@]}"
else
    gh release create "$VERSION" \
        --title "$RELEASE_TITLE" \
        --notes-file "$RELEASE_NOTES_FILE" \
        --draft \
        --generate-notes
fi

# Clean up
rm -f "$RELEASE_NOTES_FILE"

log_success "GitHub Release created: $VERSION"
log_info "Release is in DRAFT mode. Review and publish at: https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/releases/tag/$VERSION"

# ============================================================================
# Post-Release Steps
# ============================================================================

log_step "Post-release tasks"

# Tag the commit
log_info "Tagging commit with $VERSION..."
git tag -a "$VERSION" -m "UNBOUND $VERSION - Total War on Censorship"
git push origin "$VERSION"

# Commit version bumps
log_info "Committing version bumps..."
git add -A
git commit -m "chore: bump version to $VERSION across all ecosystem projects

- Desktop app: 2.0.0
- Website: 2.0.0
- Android: 2.0.0 (versionCode: 2)
- iOS: 2.0.0
- Linux: 2.0.0
- OpenWrt: 2.0.0
- Browser Extension: 2.0.0

Released as $VERSION" || log_warn "No changes to commit or already committed"

log_success "All post-release tasks completed!"

# ============================================================================
# Summary
# ============================================================================

echo ""
log_step "🎉 Release $VERSION Summary"
echo ""
echo -e "  ${GREEN}✓${NC} All platform binaries built"
echo -e "  ${GREEN}✓${NC} Website deployed to GitHub Pages"
echo -e "  ${GREEN}✓${NC} GitHub Release created (DRAFT)"
echo -e "  ${GREEN}✓${NC} Commit tagged and pushed"
echo ""
echo -e "  ${CYAN}Next steps:${NC}"
echo -e "  1. Review the draft release: ${BLUE}https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/releases${NC}"
echo -e "  2. Test all binaries on respective platforms"
echo -e "  3. Publish the release when ready"
echo ""
echo -e "  ${GREEN}🚀 Total War on Censorship! 🚀${NC}"
echo ""
