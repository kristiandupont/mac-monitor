#!/bin/bash
# Generates AppIcon.icns from the fan SVG.
# Requires: rsvg-convert (brew install librsvg), iconutil (macOS built-in)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SVG="$SCRIPT_DIR/../internal/tray/fan.svg"
ICONSET="$SCRIPT_DIR/AppIcon.iconset"

# Build a self-contained SVG with a coloured background so the strokes are
# visible at every size.  The fan path lives in a 0 0 24 24 coordinate space;
# we leave the viewBox unchanged and let rsvg-convert scale it.
ICON_SVG="$SCRIPT_DIR/AppIcon.svg"
cat > "$ICON_SVG" << 'SVG_EOF'
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
  <!-- Dark navy background — macOS applies the squircle mask automatically -->
  <rect width="24" height="24" fill="#0d1d3a"/>
  <path
    d="M10.827 16.379a6.082 6.082 0 0 1-8.618-7.002l5.412 1.45a6.082 6.082 0 0 1 7.002-8.618l-1.45 5.412a6.082 6.082 0 0 1 8.618 7.002l-5.412-1.45a6.082 6.082 0 0 1-7.002 8.618l1.45-5.412Z"
    fill="none" stroke="#e6edf3" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"/>
  <path
    d="M12 12v.01"
    fill="none" stroke="#e6edf3" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"/>
</svg>
SVG_EOF

mkdir -p "$ICONSET"

rsvg-convert -w   16 -h   16 "$ICON_SVG" -o "$ICONSET/icon_16x16.png"
rsvg-convert -w   32 -h   32 "$ICON_SVG" -o "$ICONSET/icon_16x16@2x.png"
rsvg-convert -w   32 -h   32 "$ICON_SVG" -o "$ICONSET/icon_32x32.png"
rsvg-convert -w   64 -h   64 "$ICON_SVG" -o "$ICONSET/icon_32x32@2x.png"
rsvg-convert -w  128 -h  128 "$ICON_SVG" -o "$ICONSET/icon_128x128.png"
rsvg-convert -w  256 -h  256 "$ICON_SVG" -o "$ICONSET/icon_128x128@2x.png"
rsvg-convert -w  256 -h  256 "$ICON_SVG" -o "$ICONSET/icon_256x256.png"
rsvg-convert -w  512 -h  512 "$ICON_SVG" -o "$ICONSET/icon_256x256@2x.png"
rsvg-convert -w  512 -h  512 "$ICON_SVG" -o "$ICONSET/icon_512x512.png"
rsvg-convert -w 1024 -h 1024 "$ICON_SVG" -o "$ICONSET/icon_512x512@2x.png"

iconutil -c icns -o "$SCRIPT_DIR/AppIcon.icns" "$ICONSET"

rm -rf "$ICONSET" "$ICON_SVG"
echo "Generated $SCRIPT_DIR/AppIcon.icns"
