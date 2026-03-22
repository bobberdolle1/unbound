# Unbound v1.0.1 - Critical Hotfix

## Critical Fixes

**1. GUI Profile Sync**
- All 9 profiles now properly displayed in UI (3 base + 6 advanced)
- Added diagnostic logging for profile loading failures

**2. Administrator Privileges**
- Full-screen red alert when admin rights missing
- Clear instructions: Right-click unbound.exe and Run as administrator
- WinDivert now properly blocked without privileges

**3. Auto-Tune Error Handling**
- Improved error messages with actionable instructions
- Extended error display timeout to 8 seconds
- Specific guidance: Check admin rights and internet connection

**4. UI Polish**
- Renamed Gears to Settings button
- Consistent terminology across interface

**5. Portable Embedding**
- Verified engine assets (winws2.exe, lua scripts) properly embedded
- Automatic extraction to APPDATA\Unbound\engine\ on startup

## Installation

Download Unbound-v1.0.1.exe and run with Administrator privileges.

## Upgrade Notes

If you experienced issues with v1.0.0 where profiles were not loading or admin rights were not detected, this hotfix resolves those problems.
