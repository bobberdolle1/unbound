from Quartz import CGWindowListCopyWindowInfo, kCGWindowListOptionOnScreenOnly, kCGNullWindowID

windows = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly, kCGNullWindowID)
for w in windows:
    owner = w.get('kCGWindowOwnerName', '')
    name = w.get('kCGWindowName', '')
    if 'Unbound' in owner or 'Unbound' in name:
        wid = w.get('kCGWindowNumber')
        bounds = w.get('kCGWindowBounds')
        print(f"Found Unbound window ID {wid}: bounds={bounds}")
