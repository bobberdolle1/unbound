# Release v2.0.0 Checklist

## ✅ Completed

- [x] Code implementation (DPI Engine Orchestrator + Smart Prober)
- [x] Git commit with detailed message
- [x] README.md updated with new features
- [x] Release notes created (release-notes.txt)
- [x] Git tag v2.0.0 created
- [x] Pushed to GitHub (master + tag)
- [x] Binary built (unbound-v2.0.0-windows-amd64.exe)
- [x] SHA256 checksum generated

## 📦 Release Assets

**Binary:** unbound-v2.0.0-windows-amd64.exe
**Size:** 14.57 MB
**SHA256:** 95CBA9B6D3669A9406874A67F0103B1A0AD677A455E173238E6AA3A444513AC4

## 🚀 GitHub Release Steps

1. Go to: https://github.com/bobberdolle1/unbound/releases/new
2. Select tag: v2.0.0
3. Release title: "v2.0.0 - DPI Engine Orchestrator with Smart Prober"
4. Copy content from release-notes.txt
5. Upload binary: unbound-v2.0.0-windows-amd64.exe
6. Add SHA256 checksum to release notes
7. Mark as "Latest release"
8. Publish release

## 📝 Post-Release

- [ ] Announce on project channels
- [ ] Update documentation site (if exists)
- [ ] Monitor issue tracker for bug reports
- [ ] Prepare hotfix branch if needed

## 🔧 Optional: Windows Installer

To create MSI installer (future enhancement):
```bash
# Using WiX Toolset
wails build -nsis
```

Current release uses portable .exe (no installation required)
