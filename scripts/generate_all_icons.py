import os
import shutil
import subprocess
from PIL import Image, ImageDraw

def get_project_root():
    return os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))

def sync_master_svg(root_dir):
    master = os.path.join(root_dir, "build/logo.svg")
    dest1 = os.path.join(root_dir, "frontend/src/assets/logo.svg")
    dest2 = os.path.join(root_dir, "frontend/public/logo.svg")
    
    if os.path.exists(master):
        shutil.copyfile(master, dest1)
        shutil.copyfile(master, dest2)
        print(f"Synced Master SVG: {master} -> {dest1}, {dest2}")
    else:
        print(f"Master SVG not found at {master}")

def render_logo_png(output_path, size=512, bg_color=(18, 20, 23, 255), fg_color=(255, 255, 255, 255), rx=112):
    """Render high-res PNG app icon with dark precision background and clean U-break mark."""
    scale = 4
    big_size = size * scale
    big_img = Image.new("RGBA", (big_size, big_size), (0, 0, 0, 0))
    bdraw = ImageDraw.Draw(big_img)
    bs = big_size / 512.0
    
    if bg_color is not None:
        bdraw.rounded_rectangle([0, 0, big_size-1, big_size-1], radius=int(rx * bs), fill=bg_color)
        
    bdraw.rectangle([int(96*bs), int(80*bs), int(168*bs), int(344*bs)], fill=fg_color)
    bdraw.rectangle([int(344*bs), int(240*bs), int(416*bs), int(344*bs)], fill=fg_color)
    
    brx = int(16 * bs)
    bdraw.rounded_rectangle([int(344*bs), int(80*bs), int(416*bs), int(192*bs)], radius=brx, fill=fg_color)
    
    bdraw.rounded_rectangle([int(96*bs), int(296*bs), int(416*bs), int(440*bs)], radius=int(96*bs), fill=fg_color)
    
    inner_bg = bg_color if bg_color is not None else (0, 0, 0, 0)
    bdraw.rectangle([int(168*bs), int(80*bs), int(344*bs), int(344*bs)], fill=inner_bg)
    bdraw.rounded_rectangle([int(168*bs), int(296*bs), int(344*bs), int(368*bs)], radius=int(24*bs), fill=inner_bg)

    final_img = big_img.resize((size, size), Image.Resampling.LANCZOS)
    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    final_img.save(output_path, format="PNG")
    print(f"Generated PNG: {output_path} ({size}x{size})")

def make_ico(png_path, ico_path):
    src = Image.open(png_path).convert("RGBA")
    sizes = [(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]
    os.makedirs(os.path.dirname(ico_path), exist_ok=True)
    src.save(ico_path, format="ICO", sizes=sizes)
    print(f"Generated multi-res ICO: {ico_path} (sizes: {sizes})")

def make_icns(png_512_path, icns_path, iconset_dir):
    os.makedirs(iconset_dir, exist_ok=True)
    src = Image.open(png_512_path).convert("RGBA")
    
    icon_specs = [
        (16, "icon_16x16.png"),
        (32, "icon_16x16@2x.png"),
        (32, "icon_32x32.png"),
        (64, "icon_32x32@2x.png"),
        (128, "icon_128x128.png"),
        (256, "icon_128x128@2x.png"),
        (256, "icon_256x256.png"),
        (512, "icon_256x256@2x.png"),
        (512, "icon_512x512.png"),
        (1024, "icon_512x512@2x.png")
    ]
    for sz, name in icon_specs:
        resized = src.resize((sz, sz), Image.Resampling.LANCZOS)
        resized.save(os.path.join(iconset_dir, name))
        
    res = subprocess.run(["iconutil", "-c", "icns", iconset_dir, "-o", icns_path], capture_output=True, text=True)
    if res.returncode == 0:
        print(f"Generated ICNS: {icns_path}")
    else:
        print(f"iconutil warning: {res.stderr}")

if __name__ == "__main__":
    base = get_project_root()
    
    # 1. Sync SVG master to frontend assets
    sync_master_svg(base)
    
    # 2. Render AppIcon (512x512 PNG)
    app_icon_png = os.path.join(base, "build/appicon.png")
    render_logo_png(app_icon_png, size=512, bg_color=(18, 20, 23, 255), fg_color=(255, 255, 255, 255))
    
    # 3. Favicon (PNG & ICO)
    fav_png = os.path.join(base, "frontend/public/favicon.png")
    fav_ico = os.path.join(base, "frontend/public/favicon.ico")
    render_logo_png(fav_png, size=64, bg_color=(18, 20, 23, 255), fg_color=(255, 255, 255, 255))
    make_ico(fav_png, fav_ico)
    
    # 4. Windows ICO
    win_ico = os.path.join(base, "build/windows/icon.ico")
    make_ico(app_icon_png, win_ico)
    
    # 5. macOS ICNS
    mac_icns = os.path.join(base, "build/darwin/iconfile.icns")
    mac_iconset = os.path.join(base, "build/darwin/AppIcon.iconset")
    make_icns(app_icon_png, mac_icns, mac_iconset)
