"""
Generate build/appicon.png and build/windows/icon.ico from build/logo.svg
Uses cairosvg for SVG rendering with PIL fallback.
"""
import os, sys, struct, zlib

# Try cairosvg first for proper SVG rendering
try:
    import cairosvg
    def render_svg_to_png(svg_path, png_path, size=512):
        cairosvg.svg2png(url=svg_path, write_to=png_path, output_width=size, output_height=size)
        print(f"  ✓ {png_path} ({os.path.getsize(png_path)} bytes)")
except ImportError:
    # Fallback: use PIL with basic drawing that matches the SVG
    from PIL import Image, ImageDraw
    def render_svg_to_png(svg_path, png_path, size=512):
        img = Image.new("RGBA", (size, size), (0, 0, 0, 0))
        draw = ImageDraw.Draw(img)
        s = size / 512.0
        # Background rounded rect #090d16
        rx = int(128 * s)
        draw.rounded_rectangle([0, 0, size-1, size-1], radius=rx, fill="#090d16")
        # Outer ring
        cx, cy = size // 2, size // 2
        r = int(200 * s)
        draw.ellipse([cx-r, cy-r, cx+r, cy+r], outline="#1e293b", width=max(1, int(6*s)))
        # Primary arch (white stroke)
        w1 = max(1, int(28*s))
        arch1 = [(160,336),(120,296),(120,200),(176,152),(232,104),(320,112),(352,176),(368,208),(352,256),(312,288),(272,320),(248,336),(216,336),(192,320)]
        p1 = [(int(x*s), int(y*s)) for x,y in arch1]
        draw.line(p1, fill="#f8fafc", width=w1, joint="curve")
        # Secondary arch (indigo stroke)
        w2 = max(1, int(24*s))
        arch2 = [(336,160),(376,200),(384,296),(328,344),(272,392),(184,384),(152,320),(136,288),(152,240),(192,208)]
        p2 = [(int(x*s), int(y*s)) for x,y in arch2]
        draw.line(p2, fill="#6366f1", width=w2, joint="curve")
        # Center dot
        cr = max(1, int(20*s))
        draw.ellipse([cx-cr, cy-cr, cx+cr, cy+cr], fill="#f8fafc")
        img.save(png_path, format="PNG")
        print(f"  ✓ {png_path} ({os.path.getsize(png_path)} bytes) [PIL fallback]")

def png_to_ico(png_path, ico_path, sizes=None):
    """Create multi-resolution ICO from PNG."""
    from PIL import Image
    if sizes is None:
        sizes = [16, 32, 48, 64, 128, 256]
    src = Image.open(png_path).convert("RGBA")
    imgs = [src.resize((s, s), Image.Resampling.LANCZOS) for s in sizes]
    imgs[0].save(ico_path, format="ICO", append_images=imgs[1:])
    print(f"  ✓ {ico_path} ({os.path.getsize(ico_path)} bytes, {len(imgs)} sizes)")

if __name__ == "__main__":
    os.chdir(os.path.join(os.path.dirname(__file__), ".."))
    print("Generating appicon.png...")
    render_svg_to_png("build/logo.svg", "build/appicon.png", 512)
    print("Generating icon.ico...")
    png_to_ico("build/appicon.png", "build/windows/icon.ico")
    print("Done.")
