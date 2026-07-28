#!/usr/bin/env bash
# 将完整 Noto Sans SC 子集化为水印用字体（嵌入 internal/imaging/fonts）。
# 依赖: python3 + fonttools（pip install fonttools）
#
# 用法:
#   ./scripts/subset-watermark-font.sh /path/to/NotoSansSC-Regular.otf
#   # 或把完整字体放在 fonts/NotoSansSC-Regular.full.otf 后无参运行
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FONTS="$ROOT/internal/imaging/fonts"
CHARSET="$FONTS/charset.txt"
OUT="$FONTS/NotoSansSC-Regular.otf"

SRC="${1:-}"
if [[ -z "$SRC" ]]; then
  if [[ -f "$FONTS/NotoSansSC-Regular.full.otf" ]]; then
    SRC="$FONTS/NotoSansSC-Regular.full.otf"
  else
    echo "usage: $0 /path/to/NotoSansSC-Regular.full.otf" >&2
    echo "download: https://github.com/notofonts/noto-cjk/releases (Sans SC OTF)" >&2
    exit 2
  fi
fi
if [[ ! -f "$SRC" ]]; then
  echo "missing source font: $SRC" >&2
  exit 2
fi
if [[ ! -f "$CHARSET" ]]; then
  echo "missing charset: $CHARSET" >&2
  exit 2
fi

if ! python3 -c "import fontTools" 2>/dev/null; then
  echo "fontTools required: pip install fonttools" >&2
  exit 2
fi

TMP="$(mktemp -t noto-subset.XXXXXX.otf)"
trap 'rm -f "$TMP"' EXIT

python3 -m fontTools.subset "$SRC" \
  --text-file="$CHARSET" \
  --output-file="$TMP" \
  --layout-features='*' \
  --glyph-names \
  --symbol-cmap \
  --legacy-cmap \
  --notdef-glyph \
  --notdef-outline \
  --recommended-glyphs \
  --name-IDs='*' \
  --name-legacy \
  --name-languages='*' \
  --drop-tables+=DSIG \
  --no-hinting \
  --desubroutinize

install -m 644 "$TMP" "$OUT"
echo "wrote $OUT ($(du -h "$OUT" | awk '{print $1}')) from $SRC"
echo "coverage: $(wc -m <"$CHARSET" | tr -d ' ') codepoints in charset.txt (GB2312 L1 + Latin + punct + repo CJK)"
