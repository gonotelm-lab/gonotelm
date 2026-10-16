#!/usr/bin/env python3
"""GoNoteLM 标志生成器 —— 单一几何源，产出全部 SVG 与 PNG。

标记（mark）几何是手写参数化的单一 `<path>`：无滤镜、无 mask、无渐变，
只在最外层换填充色，所以任何变体都不会走形。

依赖：仅生成 SVG 时需要标准库；生成 lockup 需要 fontTools + Geist 字体；
导出 PNG 需要 cairosvg + Pillow。已提交的产物无需重跑本脚本。

用法：
    python3 assets/logo/tools/build-logo.py            # 生成 SVG + PNG + 预览图
    python3 assets/logo/tools/build-logo.py --svg-only # 只生成 SVG
"""
from __future__ import annotations

import argparse
import os
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", ".."))
LOGO_DIR = os.path.join(ROOT, "assets", "logo")
PNG_DIR = os.path.join(LOGO_DIR, "png")

# --------------------------------------------------------------------------
# 颜色：绿色是产品/主站主色，navy 是并存的第二官方色（后端仓库与深色语境）。
# --------------------------------------------------------------------------
FOREST = "#2f6b4f"      # 主色 · studio accent
FOREST_DEEP = "#24563f"
NAVY = "#152a4a"        # 第二色 · 原 hero 主色
NAVY_DEEP = "#0f223f"
NAVY_LIGHT = "#dde4ea"
INK = "#1c2a22"         # 字标与正文色
PAPER = "#f7f4ed"       # 深色底上的反转色
WHITE = "#ffffff"

# --------------------------------------------------------------------------
# 几何：64×64 画布；实心块 5..59，右下切角，左侧三道等差槽（evenodd 挖空）。
# 槽深 34 / 25 / 16（Δ9），槽高 8.3，槽间 7.5，上下留白 7.05，槽端 R2.5。
# --------------------------------------------------------------------------
VIEWBOX = 64
SLAB = (5.0, 5.0, 54.0, 54.0)   # x, y, w, h
SLAB_R = 5.0                    # 块圆角
CHAMFER = 14.0                  # 右下切角边长
NOTCH_H = 8.3                   # 槽高
NOTCH_R = 2.5                   # 槽内端圆角
NOTCH_TOP = 7.05                # 顶/底留白
NOTCH_GUTTER = 7.5              # 槽间距
NOTCH_DEPTHS = (34.0, 25.0, 16.0)

MARK_PATH_D = None  # 由 mark_path_d() 填充


def _n(v: float) -> str:
    return f"{v:.3f}".rstrip("0").rstrip(".")


def _rounded_rect_chamfered(x, y, w, h, r, c) -> str:
    return (
        f"M{_n(x + r)} {_n(y)} H{_n(x + w - r)} "
        f"A{_n(r)} {_n(r)} 0 0 1 {_n(x + w)} {_n(y + r)} "
        f"V{_n(y + h - c)} L{_n(x + w - c)} {_n(y + h)} "
        f"H{_n(x + r)} A{_n(r)} {_n(r)} 0 0 1 {_n(x)} {_n(y + h - r)} "
        f"V{_n(y + r)} A{_n(r)} {_n(r)} 0 0 1 {_n(x + r)} {_n(y)} Z"
    )


def _notch(x, y, depth, h, r) -> str:
    return (
        f"M{_n(x)} {_n(y)} H{_n(x + depth - r)} "
        f"A{_n(r)} {_n(r)} 0 0 1 {_n(x + depth)} {_n(y + r)} "
        f"V{_n(y + h - r)} A{_n(r)} {_n(r)} 0 0 1 {_n(x + depth - r)} {_n(y + h)} "
        f"H{_n(x)} Z"
    )


def mark_path_d() -> str:
    global MARK_PATH_D
    if MARK_PATH_D:
        return MARK_PATH_D
    x, y, w, h = SLAB
    d = _rounded_rect_chamfered(x, y, w, h, SLAB_R, CHAMFER)
    top = y + NOTCH_TOP
    for i, depth in enumerate(NOTCH_DEPTHS):
        d += _notch(x, top + i * (NOTCH_H + NOTCH_GUTTER), depth, NOTCH_H, NOTCH_R)
    MARK_PATH_D = d
    return d


def mark_svg(color: str, *, title: str = "GoNoteLM") -> str:
    """标记本体。color 可为颜色值或 currentColor。"""
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {VIEWBOX} {VIEWBOX}" '
        f'width="{VIEWBOX}" height="{VIEWBOX}" role="img" aria-label="{title}" '
        f'focusable="false">\n'
        f"  <title>{title}</title>\n"
        f"  <desc>刻槽：源料被引用逐层取走</desc>\n"
        f'  <path fill="{color}" fill-rule="evenodd" d="{mark_path_d()}"/>\n'
        f"</svg>\n"
    )


# --------------------------------------------------------------------------
# 字标：Geist 600 的 "GoNoteLM" 直接转成路径，不引用外部字体。
# --------------------------------------------------------------------------
GEIST_CANDIDATES = (
    os.path.join(ROOT, "..", "gonotelm-web", "node_modules", "@fontsource", "geist",
                 "files", "geist-latin-600-normal.woff2"),
    os.path.expanduser("~/.cache/gonotelm/geist-latin-600-normal.woff2"),
)
WORDMARK = "GoNoteLM"
MARK_INK = 54.0     # 标记本身在 64 画布里的实际墨迹尺寸
MARK_SIZE = 48.0    # lockup 中标记的墨迹高度
CAP_RATIO = 0.710   # Geist sCapHeight / upem
FONT_SIZE = 44.6    # 使 cap height ≈ 标记高度的 66%
GAP = 13.0
PAD = 2.0


def _load_geist():
    from fontTools.ttLib import TTFont

    for cand in GEIST_CANDIDATES:
        if os.path.exists(cand):
            f = TTFont(cand)
            f.flavor = None
            return f
    raise FileNotFoundError(
        "找不到 Geist 600 字体；请把 geist-latin-600-normal.woff2 放到 "
        + " 或 ".join(GEIST_CANDIDATES)
    )


def wordmark_path_d() -> tuple[str, float]:
    """把 GoNoteLM 转成单条路径，返回 (path_d, 文字宽度)。"""
    from fontTools.pens.svgPathPen import SVGPathPen
    from fontTools.pens.transformPen import TransformPen
    from fontTools.misc.transform import Transform

    font = _load_geist()
    upem = font["head"].unitsPerEm
    cmap = font.getBestCmap()
    glyphset = font.getGlyphSet()
    hmtx = font["hmtx"]
    scale = FONT_SIZE / upem
    x = 0.0
    parts: list[str] = []
    for ch in WORDMARK:
        gname = cmap[ord(ch)]
        pen = SVGPathPen(glyphset)
        # 字体坐标 y 向上，SVG y 向下：翻转并在基线上结束
        tpen = TransformPen(pen, Transform(scale, 0, 0, -scale, x, 0))
        glyphset[gname].draw(tpen)
        d = pen.getCommands()
        if d:
            parts.append(d)
        x += hmtx[gname][0] * scale
    return " ".join(parts), x


def lockup_svg(mark_color: str, text_color: str, *, title: str = "GoNoteLM") -> str:
    d, text_w = wordmark_path_d()
    s = MARK_SIZE / MARK_INK
    tx = PAD - SLAB[0] * s
    ty = PAD - SLAB[1] * s
    baseline = PAD + MARK_SIZE / 2 + (CAP_RATIO * FONT_SIZE) / 2
    text_x = PAD + MARK_SIZE + GAP
    w = text_x + text_w + PAD
    h = MARK_SIZE + PAD * 2
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {_n(w)} {_n(h)}" '
        f'width="{_n(w)}" height="{_n(h)}" role="img" aria-label="{title}" '
        f'focusable="false">\n'
        f"  <title>{title}</title>\n"
        f"  <desc>GoNoteLM 横版字标：刻槽标记 + Geist 600 字标（已转路径）</desc>\n"
        f'  <g transform="translate({_n(tx)} {_n(ty)}) scale({_n(s)})">\n'
        f'    <path fill="{mark_color}" fill-rule="evenodd" d="{mark_path_d()}"/>\n'
        f"  </g>\n"
        f'  <path fill="{text_color}" d="{d}" transform="translate({_n(text_x)} {_n(baseline)})"/>\n'
        f"</svg>\n"
    )


# --------------------------------------------------------------------------
# App 图标：满幅色块 + 居中标记（iOS/PWA 需要不透明底）。
# --------------------------------------------------------------------------
APPICON_MARK = 36.0


def appicon_svg(bg: str, fg: str, *, title: str = "GoNoteLM") -> str:
    s = APPICON_MARK / MARK_INK
    off = (VIEWBOX - APPICON_MARK) / 2 - SLAB[0] * s
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {VIEWBOX} {VIEWBOX}" '
        f'width="{VIEWBOX}" height="{VIEWBOX}" role="img" aria-label="{title}" '
        f'focusable="false">\n'
        f"  <title>{title}</title>\n"
        f"  <desc>GoNoteLM App 图标</desc>\n"
        f'  <rect width="{VIEWBOX}" height="{VIEWBOX}" fill="{bg}"/>\n'
        f'  <g transform="translate({_n(off)} {_n(off)}) scale({_n(s)})">\n'
        f'    <path fill="{fg}" fill-rule="evenodd" d="{mark_path_d()}"/>\n'
        f"  </g>\n"
        f"</svg>\n"
    )


# --------------------------------------------------------------------------
def write(path: str, content: str) -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)
    print(f"  {os.path.relpath(path, ROOT)}  ({len(content.encode())} B)")


def export_png(svg: str, path: str, size: int | None = None, *,
               width: int | None = None, bg: str | None = None) -> None:
    """按方形边长（size）或按宽度等比（width）导出 PNG，二者取其一。"""
    import cairosvg

    os.makedirs(os.path.dirname(path), exist_ok=True)
    kwargs: dict = {"background_color": bg}
    if width is not None:
        kwargs["output_width"] = width
        label = f"{width}px wide"
    else:
        kwargs["output_width"] = size
        kwargs["output_height"] = size
        label = f"{size}px"
    cairosvg.svg2png(bytestring=svg.encode(), write_to=path, **kwargs)
    print(f"  {os.path.relpath(path, ROOT)}  ({label})")


def render_preview() -> None:
    """assets/logo/preview.png —— 给人类看的品牌总览页。"""
    import cairosvg
    from PIL import Image, ImageDraw, ImageFont

    W, H = 1180, 1150
    im = Image.new("RGB", (W, H), "#efebe2")
    dr = ImageDraw.Draw(im)
    try:
        f = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 21)
        fs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 15)
    except OSError:  # 无字体时退化为默认位图字体
        f = fs = ImageFont.load_default()
    tmp = os.path.join(PNG_DIR, "_tmp.png")

    def paste_svg(svg: str, px: int, xy: tuple[int, int], bg: str | None) -> None:
        cairosvg.svg2png(bytestring=svg.encode(), write_to=tmp,
                         output_width=px, output_height=px, background_color=bg)
        img = Image.open(tmp).convert("RGBA")
        base = Image.new("RGB", (px, px), bg or "#efebe2")
        base.paste(img, (0, 0), img)
        im.paste(base, xy)

    dr.text((24, 18), "GoNoteLM — logo / 刻槽", fill="#16181c", font=f)
    dr.text((24, 48), "solid source block, notched by citations · one path · no filters",
            fill="#7a7a7a", font=fs)

    dr.text((24, 84), "mark — colorways", fill="#7a7a7a", font=fs)
    paste_svg(mark_svg(FOREST), 150, (24, 108), None)
    paste_svg(mark_svg(NAVY), 150, (196, 108), None)
    paste_svg(mark_svg(PAPER), 150, (368, 108), NAVY)
    paste_svg(mark_svg(INK), 150, (540, 108), None)
    for i, label in enumerate(("forest #2f6b4f", "navy #152a4a", "reverse on navy", "ink #1c2a22")):
        dr.text((24 + i * 172, 262), label, fill="#16181c", font=fs)

    dr.text((24, 296), "size ladder — 16 / 24 / 32 / 48 / 64 px, actual pixels",
            fill="#7a7a7a", font=fs)
    for row, bg in enumerate(("#ffffff", "#14161a")):
        ink = FOREST if row == 0 else PAPER
        x = 24
        y = 320 + row * 84
        for px in (16, 24, 32, 48, 64):
            paste_svg(mark_svg(ink), px, (x, y + 72 - px), bg)
            x += px + 26

    dr.text((24, 496), "lockup — Geist 600 converted to paths", fill="#7a7a7a", font=fs)
    for i, (svg, bg) in enumerate(((lockup_svg(FOREST, INK), "#f7f4ed"),
                                   (lockup_svg(NAVY, NAVY), "#ffffff"),
                                   (lockup_svg(PAPER, PAPER), "#0f223f"))):
        lk = os.path.join(PNG_DIR, "_lk.png")
        cairosvg.svg2png(bytestring=svg.encode(), write_to=lk,
                         output_width=620, background_color=bg)
        img = Image.open(lk).convert("RGB")
        y = 520 + i * 124
        im.paste(img, (24, y))
        dr.rectangle([24, y, 24 + img.width - 1, y + img.height - 1], outline="#d8d2c6")

    dr.text((700, 496), "app icon", fill="#7a7a7a", font=fs)
    for i, (svg, bg) in enumerate(((appicon_svg(FOREST, PAPER), FOREST),
                                   (appicon_svg(NAVY, NAVY_LIGHT), NAVY))):
        cairosvg.svg2png(bytestring=svg.encode(), write_to=tmp,
                         output_width=96, output_height=96, background_color=bg)
        ic = Image.open(tmp).convert("RGB")
        mask = Image.new("L", (96, 96), 0)
        ImageDraw.Draw(mask).rounded_rectangle([0, 0, 95, 95], 22, fill=255)
        im.paste(ic, (700 + i * 118, 520), mask)

    dr.text((700, 650), "palette", fill="#7a7a7a", font=fs)
    swatches = (("forest", FOREST), ("forest-deep", FOREST_DEEP), ("navy", NAVY),
                ("navy-deep", NAVY_DEEP), ("navy-light", NAVY_LIGHT), ("ink", INK),
                ("paper", PAPER))
    for i, (name, hexv) in enumerate(swatches):
        y = 674 + i * 34
        dr.rectangle([700, y, 736, y + 26], fill=hexv, outline="#d8d2c6")
        dr.text((746, y + 4), f"{name}   {hexv}", fill="#16181c", font=fs)

    dr.text((700, 930), "clear space — 0.25 × mark height", fill="#7a7a7a", font=fs)
    box = 112
    pad = box // 4
    frame = Image.new("RGB", (box + pad * 2, box + pad * 2), "#ffffff")
    cairosvg.svg2png(bytestring=mark_svg(FOREST).encode(), write_to=tmp,
                     output_width=box, output_height=box, background_color="#ffffff")
    frame.paste(Image.open(tmp).convert("RGBA"), (pad, pad), Image.open(tmp).convert("RGBA"))
    ImageDraw.Draw(frame).rectangle([0, 0, frame.width - 1, frame.height - 1], outline="#c9c2b4")
    im.paste(frame, (700, 956))
    dr.text((24, 1090), "assets/logo · regenerate: python3 assets/logo/tools/build-logo.py",
            fill="#9a9a9a", font=fs)

    out = os.path.join(LOGO_DIR, "preview.png")
    im.save(out)
    for leftover in ("_tmp.png", "_lk.png"):
        p = os.path.join(PNG_DIR, leftover)
        if os.path.exists(p):
            os.remove(p)
    print(f"  {os.path.relpath(out, ROOT)}")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--svg-only", action="store_true", help="不导出 PNG")
    args = ap.parse_args()

    print("标记与字标：")
    write(os.path.join(LOGO_DIR, "gonotelm-mark-forest.svg"), mark_svg(FOREST))
    write(os.path.join(LOGO_DIR, "gonotelm-mark-navy.svg"), mark_svg(NAVY))
    write(os.path.join(LOGO_DIR, "gonotelm-mark-mono.svg"), mark_svg("currentColor"))
    write(os.path.join(LOGO_DIR, "gonotelm-mark-reverse.svg"), mark_svg(PAPER))
    # 主站 favicon：绿色透明底；dark 版用于深色标签栏
    write(os.path.join(LOGO_DIR, "favicon.svg"), mark_svg(FOREST))
    write(os.path.join(LOGO_DIR, "favicon-dark.svg"), mark_svg(PAPER))
    write(os.path.join(LOGO_DIR, "gonotelm-lockup-forest.svg"), lockup_svg(FOREST, INK))
    write(os.path.join(LOGO_DIR, "gonotelm-lockup-navy.svg"), lockup_svg(NAVY, NAVY))
    write(os.path.join(LOGO_DIR, "gonotelm-lockup-mono.svg"), lockup_svg("currentColor", "currentColor"))
    write(os.path.join(LOGO_DIR, "gonotelm-lockup-reverse.svg"), lockup_svg(PAPER, PAPER))
    write(os.path.join(LOGO_DIR, "gonotelm-appicon-forest.svg"), appicon_svg(FOREST, PAPER))
    write(os.path.join(LOGO_DIR, "gonotelm-appicon-navy.svg"), appicon_svg(NAVY, NAVY_LIGHT))

    # 本仓库 README 顶图：只放标记本体（navy），深色主题用浅色版，二者同源同形。
    write(os.path.join(ROOT, "docs", "readme", "hero.svg"), mark_svg(NAVY))
    write(os.path.join(ROOT, "docs", "readme", "hero-dark.svg"), mark_svg(NAVY_LIGHT))

    if args.svg_only:
        return 0

    print("PNG 导出：")
    try:
        import cairosvg  # noqa: F401
    except ImportError:
        print("  跳过：未安装 cairosvg")
        return 0
    for size in (16, 32, 48, 512):
        export_png(mark_svg(FOREST), os.path.join(PNG_DIR, f"favicon-{size}.png"), size)
    export_png(appicon_svg(FOREST, PAPER),
               os.path.join(PNG_DIR, "apple-touch-icon-180.png"), 180)
    export_png(mark_svg(FOREST), os.path.join(PNG_DIR, "mark-forest-512.png"), 512)
    export_png(mark_svg(NAVY), os.path.join(PNG_DIR, "mark-navy-512.png"), 512)
    export_png(mark_svg(PAPER), os.path.join(PNG_DIR, "mark-reverse-512.png"), 512)
    export_png(appicon_svg(NAVY, NAVY_LIGHT), os.path.join(PNG_DIR, "appicon-navy-512.png"), 512)
    for name, lk in (("forest", lockup_svg(FOREST, INK)),
                     ("navy", lockup_svg(NAVY, NAVY)),
                     ("reverse", lockup_svg(PAPER, PAPER))):
        export_png(lk, os.path.join(PNG_DIR, f"lockup-{name}-1200.png"), width=1200)
    # README 顶图只放标记本体，横版字标 PNG 作为素材常备（幻灯片、社交预览、文档封面）。
    export_png(lockup_svg(NAVY, NAVY),
               os.path.join(ROOT, "docs", "readme", "hero-lockup.png"), width=1200)
    export_png(lockup_svg(PAPER, PAPER),
               os.path.join(ROOT, "docs", "readme", "hero-lockup-dark.png"), width=1200)
    print("品牌总览页：")
    render_preview()
    return 0


if __name__ == "__main__":
    sys.exit(main())
