import subprocess, time, os

def get_unbound_window_id():
    cmd = """osascript -e '
    tell application "System Events"
        set winID to ""
        tell process "Unbound"
            set frontmost to true
            try
                set winID to value of attribute "AXWindow" of window 1
            end try
        end tell
    end tell
    return winID'"""
    res = subprocess.run(cmd, shell=True, capture_output=True, text=True)
    return res.stdout.strip()

# Create screenshots output directory
os.makedirs("/Users/kirill/Documents/unbound/docs/assets/screenshots", exist_ok=True)

# Capture current window state
out_file = "/Users/kirill/Documents/unbound/docs/assets/screenshots/monolith-connected.png"
subprocess.run(["screencapture", "-x", "-R", "200,200,760,520", out_file])
print(f"Captured screenshot to {out_file}")
