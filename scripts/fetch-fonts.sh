#!/usr/bin/env bash
# Regenerate frontend/src/fonts.css and frontend/src/fonts/*.woff2 from Google
# Fonts.
#
# The desktop UI must render correctly with no internet access: Unbound is run
# precisely when the connection is censored or broken. Linking to
# fonts.googleapis.com at runtime meant the themed UI silently fell back to
# system fonts exactly when it mattered, and contradicted the project's claim
# of making no outbound requests.
#
# Run this only when the font list in the UI changes.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FONT_DIR="$REPO_ROOT/frontend/src/fonts"
CSS_OUT="$REPO_ROOT/frontend/src/fonts.css"

QUERY='family=Inter:wght@400;500;600;700&family=Comic+Neue:wght@400;700&family=Permanent+Marker&family=VT323&display=swap'
# Google serves different files per User-Agent; a modern UA yields woff2.
UA='Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'

TMP_CSS="$(mktemp)"
trap 'rm -f "$TMP_CSS"' EXIT

echo "==> Fetching @font-face declarations"
curl -sSf -A "$UA" "https://fonts.googleapis.com/css2?${QUERY}" -o "$TMP_CSS"

mkdir -p "$FONT_DIR"
rm -f "$FONT_DIR"/*.woff2

echo "==> Downloading woff2 files"
python3 - "$TMP_CSS" "$FONT_DIR" "$CSS_OUT" <<'PY'
import re, subprocess, sys

src_css, font_dir, css_out = sys.argv[1], sys.argv[2], sys.argv[3]
css = open(src_css).read()

# The UI ships Russian and English copy only. Shipping the greek/vietnamese
# subsets would roughly double the bundled weight for glyphs never rendered.
KEEP = {"latin", "latin-ext", "cyrillic", "cyrillic-ext"}

HEADER = """/* Bundled locally so the UI renders correctly offline and behind a censored
   connection - i.e. the exact conditions Unbound is used in. Fetching these
   from fonts.googleapis.com also contradicted the project's "no outbound
   requests" claim.

   Sources: Google Fonts. Inter, Comic Neue and VT323 are OFL-1.1;
   Permanent Marker is Apache-2.0. Regenerate with scripts/fetch-fonts.sh -
   do not hand-edit. */

"""

blocks = re.findall(r"/\* (\S+) \*/\n(@font-face \{.*?\n\})", css, re.S)
if not blocks:
    sys.exit("no @font-face blocks parsed - did the Google Fonts CSS format change?")

rules, downloaded = [], {}
for subset, block in blocks:
    if subset not in KEEP:
        continue
    family = re.search(r"font-family: '([^']+)'", block).group(1)
    weight = re.search(r"font-weight: (\d+)", block).group(1)
    url = re.search(r"url\((https://[^)]+)\)", block).group(1)
    if url not in downloaded:
        name = f"{family.lower().replace(' ', '-')}-{weight}-{subset}.woff2"
        subprocess.run(["curl", "-sSf", "-o", f"{font_dir}/{name}", url], check=True)
        downloaded[url] = name
    rules.append(f"/* {subset} */\n" + block.replace(url, f"./fonts/{downloaded[url]}"))

open(css_out, "w").write(HEADER + "\n\n".join(rules) + "\n")
print(f"    {len(downloaded)} woff2 files, {len(rules)} @font-face rules")
PY

echo "==> Wrote $CSS_OUT"
