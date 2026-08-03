import os, subprocess
from PIL import Image, ImageDraw

def create_mac_appicon(svg_path, out_png, size=512, bg_hex="#121417", fg_hex="#ffffff"):
    # 1. Render SVG cleanly using qlmanage at target size
    tmp_svg = "/tmp/logo_white.svg"
    with open(svg_path, "r") as f:
        content = f.read()
    content = content.replace("currentColor", fg_hex)
    with open(tmp_svg, "w") as f:
        f.write(content)
        
    subprocess.run(["qlmanage", "-t", "-s", str(int(size*0.65)), "-o", "/tmp", tmp_svg], capture_output=True)
    rendered_png = "/tmp/logo_white.svg.png"
    
    mark_img = Image.open(rendered_png).convert("RGBA")
    
    # 2. Create rounded background canvas
    canvas = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(canvas)
    
    # Parse hex color
    h = bg_hex.lstrip('#')
    bg_rgb = tuple(int(h[i:i+2], 16) for i in (0, 2, 4)) + (255,)
    
    rx = int(size * 0.22)
    draw.rounded_rectangle([0, 0, size-1, size-1], radius=rx, fill=bg_rgb)
    
    # Center the mark
    mw, mh = mark_img.size
    ox = (size - mw) // 2
    oy = (size - mh) // 2
    canvas.paste(mark_img, (ox, oy), mark_img)
    canvas.save(out_png, format="PNG")
    print(f"Created pixel-perfect icon: {out_png}")

if __name__ == "__main__":
    base = "/Users/kirill/Documents/unbound"
    svg = os.path.join(base, "build/logo.svg")
    appicon = os.path.join(base, "build/appicon.png")
    create_mac_appicon(svg, appicon, size=512)
    
    # Favicon
    favicon = os.path.join(base, "frontend/public/favicon.png")
    create_mac_appicon(svg, favicon, size=64)
    
    # Re-generate ICO and ICNS
    from generate_all_icons import make_ico, make_icns
    make_ico(appicon, os.path.join(base, "build/windows/icon.ico"))
    make_ico(favicon, os.path.join(base, "frontend/public/favicon.ico"))
    make_icns(appicon, os.path.join(base, "build/darwin/iconfile.icns"))
