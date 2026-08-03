import subprocess, json

cmd = """osascript -e '
tell application "System Events"
    tell process "Unbound"
        set frontmost to true
        set win to window 1
        set {x, y} to position of win
        set {w, h} to size of win
        return (x as text) & "," & (y as text) & "," & (w as text) & "," & (h as text)
    end tell
end tell'"""

res = subprocess.run(cmd, shell=True, capture_output=True, text=True)
print("Unbound Window Bounds:", res.stdout.strip())
