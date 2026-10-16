# GoNoteLM 标志

**概念：刻槽。** 一块实心的"源料"，被引用从左逐层取走——取走多少，留下多少，一眼看得见。
三道槽等差递减（34 / 25 / 16，Δ9），右下切角破了正方形的轮廓，同时把偏右的质量重心拉回几何中心。

设计过程（12 个备选方向）与本次取舍的过程文件**未入库**；标记的最终几何以本文件与
`tools/build-logo.py` 为准，全部产物都可由它重新生成。

## 几何规范

标记是**单一 `<path>`**，靠 `fill-rule="evenodd"` 挖空，改颜色只改 `fill`，任何变体都不会走形。

| 项 | 值 |
|---|---|
| 画布 | `viewBox="0 0 64 64"` |
| 实心块 | `x=5 y=5 w=54 h=54`，圆角 `R5` |
| 右下切角 | `14`（识别点之一，不可缩小） |
| 槽 | 3 道，从左边 `x=5` 切入；槽高 `8.3`，内端圆角 `R2.5` |
| 槽深 | `34 / 25 / 16`（等差 Δ9） |
| 槽间距 / 上下留白 | `7.5` / `7.05` |
| 结构 | 1 个 `<path>`、4 个子路径、无滤镜 / 无 mask / 无渐变 / 无外部字体或位图 |
| 体积 | 标记 577 B；横版字标 5.5 KB（字标已转路径） |

实测指标：单色墨色覆盖 **55.7%**；16 / 24 / 32 px 均清晰，**不需要 16px 专用第二形态**；
若极端环境下槽线发糊，回退方案是同一几何砍到 2 槽（槽距 11），不是重画。

## 颜色

绿色是**主色**（产品与主站），navy 是**并存的第二官方色**（后端仓库、API 文档）。同一个画面里只用一套。

| 名称 | 值 | 用途 |
|---|---|---|
| forest | `#2f6b4f` | 主色：主站 favicon、登录页、官网 |
| forest-deep | `#24563f` | forest 深一档（hover / 深底） |
| navy | `#152a4a` | 第二色：本仓库 README、后端文档 |
| navy-deep | `#0f223f` | navy 体系的深底 |
| navy-light | `#dde4ea` | navy 体系的反转色 |
| ink | `#1c2a22` | 与 forest 搭配的字标色 |
| paper | `#f7f4ed` | 深色底上的反转色 |

## 文件

| 文件 | 用途 |
|---|---|
| `gonotelm-mark-forest.svg` | 主标记，绿色（产品默认） |
| `gonotelm-mark-navy.svg` | 第二色标记 |
| `gonotelm-mark-mono.svg` | `fill="currentColor"`，继承 CSS 文字色，单色印刷 / 任意底色 |
| `gonotelm-mark-reverse.svg` | 浅色标记，用于深色底 |
| `gonotelm-lockup-{forest,navy,mono,reverse}.svg` | 横版字标：标记 + Geist 600 字标（已转路径） |
| `gonotelm-appicon-{forest,navy}.svg` | App 图标：满幅色块 + 居中标记 |
| `favicon.svg` / `favicon-dark.svg` | 主站 favicon 的浅色底 / 深色标签栏两版 |
| `png/` | 16 / 32 / 48 / 512 favicon、180 apple-touch-icon、512 标记、1200 宽字标（forest / navy / reverse） |
| `preview.png` | 品牌总览页（颜色、尺寸阶梯、lockup、App 图标、安全边距） |

## 用法

- **最小尺寸 16px**；**安全边距 = 0.25 × 标记高度**，四边等距，不得放文字或其他图形。
- **深色背景**用 `-reverse`（或 `navy-light`），**浅色背景**用 forest / navy 原色。
- **继承文字色**用 `-mono`（`currentColor`）。
- 主站 favicon 已经是 `gonotelm-web/public/favicon.svg`，深色标签栏走 `favicon-dark.svg`，
  两者都在 `index.html` 里由 `media="(prefers-color-scheme: dark)"` 切换。
- README 顶图用**横版字标**：`docs/readme/hero-lockup.png`（navy）与 `hero-lockup-dark.png`（paper）
  由 `<picture>` + `prefers-color-scheme` 切换。PNG 不能像 `currentColor` 那样跟随主题换色，
  所以深色主题必须单独出一张——**不要只放浅色那张**，GitHub 深色下 navy 会糊进背景。
- 字标 PNG 一律**等比缩放**（1200 × 210，5.71:1）；导出时只给宽度，不要同时写死宽高。

## 禁止

- 不要非等比拉伸、旋转、加阴影 / 描边 / 渐变 / 发光——标记的辨识度来自轮廓，任何装饰都会稀释它。
- 不要改动槽数（3）与切角比例（14），这两个是它的识别点。
- 不要在浅色底上用 `-reverse`，也不要在深色底上直接用 forest / navy。
- 不要重新排版字标：字标是 Geist 600 转路径的结果，换字必须重跑生成脚本。

## 重新生成

```bash
python3 assets/logo/tools/build-logo.py            # SVG + PNG + preview.png
python3 assets/logo/tools/build-logo.py --svg-only # 只重建 SVG
```

依赖：`fontTools`（字标转路径）、`cairosvg` + `Pillow`（导出 PNG 与预览图）。
字标依赖 Geist 600 的 `geist-latin-600-normal.woff2`（默认从同级仓库 `gonotelm-web`
的 `node_modules` 读取）。产物已入库，日常改色 / 换尺寸**不需要**重跑脚本，直接编辑 SVG 的 `fill` 即可。
