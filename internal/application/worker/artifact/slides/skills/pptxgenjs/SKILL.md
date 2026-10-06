---
name: pptxgenjs
description: PptxGenJS 4.0.1 的 API 与陷阱：文本/要点/形状/表格/图表/图片/演讲者备注的正确写法、会损坏文件或静默失效的选项、compile.js 的运行约定。写 slide-*.js 或 compile.js 前必读。
---

# PptxGenJS 4.0.1（本沙箱唯一可用库）

本技能是 slides 生成入口提示的知识库（位于工作目录 `skills/pptxgenjs/SKILL.md`）。
沙箱里的 `vendor/standalone.cjs` 就是官方 pptxgenjs 4.0.1 的完整打包（无删减、无改名，接口与官方文档一致）。

**不要联网、不要 npm/npx、不要另找或替换库。** 下方未覆盖的选项，先按本文件的写法推，再靠 `node` 报错定位；不要去探测文件系统。

## 运行约定

```javascript
const path = require("path");
const pptxgen = require(path.join(__dirname, "..", "vendor", "standalone.cjs"));
// 主题 token（唯一来源，只读）：颜色 / 字体 / 字号 / 间距 / 圆角
const tokens = require(path.join(__dirname, "..", "skills", "theme", "<visual_style>", "theme.json"));
const theme = tokens.palette;
```

- `new pptxgen()`：**每个演示文稿一个新实例**，禁止复用。
- `pres.layout = "LAYOUT_16x9"` 必须在任何 `addSlide()` **之前**设置。
- 每页模块导出 `createSlide(pres, theme, tokens)`，**必须同步**（禁止 async）；由 `compile.js` 顺序调用。
- `theme` = `tokens.palette`，`tokens` = `theme.json` 全文；**颜色/字体/字号/间距/圆角一律取 `tokens`，禁止写死**（用法见 `slides-design`）。
- `await pres.writeFile({ fileName })` **必须 await**，否则进程可能在写盘前退出。

## Slide 模块骨架（每页一个文件）

```javascript
// <workspace>/slides/slide-01.js
const slideConfig = {
  type: "content", // cover | toc | section | content | summary
  index: 1,
  title: "本页标题"
};

function createSlide(pres, theme, tokens) { // 必须同步，禁止 async
  const slide = pres.addSlide();
  slide.background = { color: theme.bg };

  // ① 画布与边距 → ② 游标与预算 → ③ 可行性门闩
  // 常量与公式照抄 slides-layout 的代码块；比例取自 tokens.space / tokens.radius，字号取自 tokens.type
  // 门闩通过后再 addText / addShape；页码由页脚带推导（封面除外）

  return slide;
}

module.exports = { createSlide, slideConfig };
```

- 编号 `slide-01.js … slide-0N.js` **连续**；`compile.js` 按 `totalSlides` 顺序 require 并调用。
- **一页一个文件（强制）**：禁止把多页合并进一个文件，也禁止「页面数据数组 + 循环 `addSlide()`」的批量写法——页码与版式差异会退化成一堆分支，而且一次生成过大容易超时/被截断。
- 单页文件控制在 **≤ ~120 行 / ~8 KB**。写不下说明内容太多 → 减字或拆页，不要把多页挤进来。
- 每轮并行写 **2–4 页**（每次 `WriteFile` 只写一页）；编译报错定位到哪一页，就只重写或 `EditFile` 那一页。
- `slideConfig` 只记录本页元信息（页型 / 序号 / 标题），供自查一致性用，不参与渲染。
- 一页一次写完整（含 `module.exports`），靠覆盖纠错；不要多轮零散 `EditFile`。

## 画布与单位

| layout | 尺寸（英寸） | 说明 |
|---|---|---|
| `LAYOUT_16x9` | **10 × 5.625** | 本流程固定使用（默认值） |
| `LAYOUT_WIDE` | 13.333 × 7.5 | 本流程不用 |

- 所有 `x/y/w/h` 单位是**英寸**（数值字面量），不要按 13.3 宽的画布算。
- ⚠️ 超出画布的坐标**不报错、也不被裁剪**：图形只是没画在页面上，等于静默丢失。必须自己保证 `x + w ≤ 10`、`y + h ≤ 5.625`。

## 颜色

- 6 位 hex 字符串，**不带 `#`**：`color: "E07A5F"`。
- ⚠️ 8 位 hex（把透明度塞进 hex，如 `"00000020"`）会**损坏文件**。
- 半透明：填充用 `fill: { color, transparency: 0-100 }`，阴影用 `shadow: { opacity: 0-1 }`；两者不能互换，写错位置会被**静默忽略**。
- 渐变填充不支持 → 用纯色、半透明叠色或本地图片做背景。

## 文本 addText

```javascript
slide.addText("文本", {
  x, y, w, h,
  fontSize: tokens.type.body, fontFace: tokens.fonts.latin, color: theme.secondary, bold: false,
  align: "left", valign: "middle", margin: 0
});
```

常用 options：`fontSize` `fontFace` `bold` `italic` `underline` `color` `align` `valign` `margin` `wrap` `fit` `rotate` `isTextBox` `charSpacing` `lineSpacing` `paraSpaceAfter` `transparency` `shadow`。

- 文字超出 `w/h` 不会被裁剪，但 PPT 里会溢出压到别的元素 → 排版必须自己算得下。
- 与图形/线条对齐时 `margin: 0`（文本框自带内边距，不归零就永远差几个点）。
- 富文本数组：多行时**除最后一项外都要 `breakLine: true`**。

```javascript
slide.addText([
  { text: "第一行", options: { breakLine: true } },
  { text: "加粗 ", options: { bold: true } },
  { text: "常规" }
], { x, y, w, h });
```

- ⚠️ `letterSpacing` 不存在（写上去静默忽略），字距用 **`charSpacing`**。
- 文本里手写 `*` / `•` / `o` 不是列表标记，只会多出一个字面符号；列表请用 `bullet`。

## 列表与要点

```javascript
slide.addText([
  { text: "第一条", options: { bullet: true, breakLine: true } },
  { text: "第二条", options: { bullet: true, breakLine: true } },
  { text: "第三条", options: { bullet: true } }
], { x, y, w, h });
```

- 每项 `bullet: true`；**最后一项不要 `breakLine`**。
- 子级用 `indentLevel: 1`；编号列表用 `bullet: { type: "number" }`。
- 要点之间的间距用 `paraSpaceAfter`；**不要**与 `bullet` 同时用 `lineSpacing`（会撑出巨大空隙）。

## 形状

```javascript
slide.addShape(pres.shapes.RECTANGLE, { x, y, w, h, fill: { color }, line: { color, width, dashType } });
slide.addShape(pres.shapes.OVAL, { x, y, w, h, fill: { color } });
slide.addShape(pres.shapes.ROUNDED_RECTANGLE, { x, y, w, h, fill: { color }, rectRadius: 卡片短边 * 0.06 });
slide.addShape(pres.shapes.LINE, { x, y, w, h: 0, line: { color, width: 1, dashType: "dash" } });
```

- `pres.shapes` 有 184 种（`RECTANGLE` / `ROUNDED_RECTANGLE` / `OVAL` / `LINE` / `CHEVRON` / `BLOCK_ARC` / `ARC` …）；不确定的名字先 `grep -o "CHEVRON"` 式自查，别乱猜。
- ⚠️ `rectRadius` 只对 `ROUNDED_RECTANGLE` 生效。
- 阴影：`shadow: { type: "outer", color: "000000", blur: 6, offset: 2, angle: 135, opacity: 0.15 }`。
  ⚠️ `offset` 必须 **≥ 0**（负值损坏文件）；要向上投影用 `angle: 270` + 正 offset。
- 旋转/镜像：`rotate: 90`、`flipH: true`、`flipV: true`。

## 图片

```javascript
slide.addImage({ path: "imgs/chart.png", x, y, w, h });
slide.addImage({ data: "image/png;base64," + b64, x, y, w, h }); // base64 前缀必需
```

- 等比缩放自己算：`w = 目标高 * (原宽 / 原高)`；支持 PNG / JPG / GIF / SVG。
- 沙箱**无网络**：不要引用 http 图片，不要装 `sharp` / `react-icons` / `image-size`。没有现成图片时就别放图（用图表/形状/图标形状代替）。

## 表格

```javascript
slide.addTable([
  ["表头", "表头"],
  ["单元格", "单元格"]
], {
  x, y, w, h, colW: [3, 2], rowH: 0.4,
  border: { pt: 1, color: "999999" },
  fill: { color: "FFFFFF" },
  fontFace: tokens.fonts.latin, fontSize: tokens.type.caption, color: theme.primary, valign: "middle"
});
```

- 单元格可写成对象：`{ text: "合计", options: { bold: true, colspan: 2, align: "right" } }`。
- `colW` 各项之和应等于 `w`；行高与内容冲突时以内容为准，注意总高别越过内容区底边。

## 图表

```javascript
slide.addChart(pres.charts.BAR, [
  { name: "销量", labels: ["Q1", "Q2", "Q3"], values: [12, 18, 31] }
], {
  x, y, w, h, barDir: "col",
  showTitle: true, title: "季度销量", titleColor: "3D405B", titleFontSize: 12,
  showValue: true, dataLabelPosition: "outEnd", dataLabelColor: "1D3557",
  chartColors: tokens.chart.colors,                 // 只用主题里的图表色
  catAxisLabelColor: tokens.chart.axis, valAxisLabelColor: tokens.chart.axis,
  valGridLine: { color: tokens.chart.grid, size: 0.5 }, catGridLine: { style: "none" },
  showLegend: false,
  chartArea: { fill: { color: "FFFFFF" }, roundedCorners: true }
});
```

- 类型（`pres.charts.*`，共 10 种）：`AREA` `BAR` `BAR3D` `BUBBLE` `BUBBLE3D` `DOUGHNUT` `LINE` `PIE` `RADAR` `SCATTER`。
- 默认图表是"裸"的（无标题、无数据标签、灰调色板）。至少设 `showTitle` + `title`、`showValue` + `dataLabelPosition`、`chartColors: tokens.chart.colors`、轴标签色、`showLegend`，否则一律算未完成。
- ⚠️ 堆叠柱/条（`barGrouping: "stacked"`）时 `dataLabelPosition` 只能是 `ctr` / `inBase` / `inEnd`；用 `outEnd` 会**损坏文件**。
- ⚠️ 组合图传数组：`[{ type: pres.charts.BAR, data: [...], options: {...} }, { type: pres.charts.LINE, data: [...], options: {...} }]`；需要双轴时，`secondaryValAxis` / `secondaryCatAxis` 必须**同时**给 `valAxes` 与 `catAxes`，各两项——只给 `valAxes` 会让 PowerPoint 判定文件损坏并丢弃该图表。
- 库不暴露的 PowerPoint 原生特性（趋势线、误差线）：自己算成额外数据系列，**不要**退回贴图。

## 演讲者备注（必须使用）

```javascript
slide.addNotes("细节、背景、展开论述、数据出处");
```

- 纯文本，一页调用一次。
- 大纲里的 `> 备注：` 内容**必须**落在这里；它是给讲者的，**不要**画到页面上，也不要丢弃。

## 母版与页码

```javascript
pres.defineSlideMaster({
  title: "BASE",
  background: { color: theme.bg },
  objects: [
    { text: { text: "页脚文字", options: { x: 0.3, y: 5.25, w: 3, h: 0.3, fontSize: 9, color: theme.secondary } } }
  ],
  slideNumber: { x: 9.35, y: 5.25, w: 0.35, h: 0.25, color: theme.accent, fontSize: 10, align: "right" }
});
const slide = pres.addSlide({ masterName: "BASE" });
```

- 母版适合重复出现的页脚/装饰/页码；本流程若要求"页码胶囊由本页布局常量推导"，就仍按 `slides-layout` 的页脚带规则在每页自算，二者选一，全文统一。

## 会损坏文件的清单（逐条避开）

1. 8 位 hex 颜色（把 alpha 塞进 hex）。
2. `shadow.offset` 为负。
3. 堆叠柱/条 `dataLabelPosition: "outEnd"`。
4. 双轴组合图只给 `valAxes` 不给 `catAxes`。
5. **复用同一个 options 对象**跨多次 `add*`：pptxgenjs 会**原地改写**（如把 shadow 转成 EMU），第二次调用就变形。每次新建对象或用工厂函数。
6. 把整个对象当 `color` 传（会触发 `(colorStr || "").replace` 报错）：`color` / `fill.color` 必须是 hex 字符串。
7. 手工改 `pres.slides[i]` 内部结构或 `presLayout`。

## 静默失效清单（不报错但没效果）

- `letterSpacing`（正确名字是 `charSpacing`）
- 渐变填充
- 超出画布的坐标（图形直接不在页面上）
- 动画与切换（库不支持，本流程也禁止）
- 把 `rectRadius` 用在 `RECTANGLE` 上
- `transparency` / `opacity` 用错位置（填充 vs 阴影）
- `lineSpacing` 与 `bullet` 混用
