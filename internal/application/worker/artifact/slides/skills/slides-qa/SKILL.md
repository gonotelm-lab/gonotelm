---
name: slides-qa
description: 交稿前自检：本沙箱能验什么/不能验什么、用 unzip 抽查页面与备注内容、逐页重算坐标排查重叠与溢出、编译与 CheckPPTX 的收尾流程。调用 CheckPPTX 前后读。
---

# 交稿前自检（未通过不得结束）

本技能是 slides 生成入口提示的知识库（位于工作目录 `skills/slides-qa/SKILL.md`）。

## 0. 先搞清楚这个沙箱能验什么

**能用的**：`node`、`unzip`、`grep` / `sed`、以及 `CheckPPTX` 工具。

**没有的**（不要尝试，浪费轮次）：

- ❌ LibreOffice / `soffice`、`pdftoppm`、`markitdown`——**本流程不做视觉渲染 QA**；
- ❌ 网络、`npm` / `npx` / `pip`、任何校验脚本下载；
- ❌ `CheckPPTX` 只看文件**是不是** pptx（读开头几 KB 做 MIME 嗅探，**完全不解析 XML**）。它返回 OK ≠ 排版没问题，也 ≠ 内容都在。

结论：**版式的正确性只能靠静态审计**（`slides-layout` 的"先算后画" + 下面第 2 节逐页重算），内容的完整性靠第 1 节的 XML 抽查。

## 1. 内容审计（用 unzip 抽查产物）

```bash
OUT=slides/output/presentation.pptx

# 页数（应等于 compile.js 的 totalSlides）
unzip -l "$OUT" | grep -cE "ppt/slides/slide[0-9]+\.xml$"

# 第 N 页的可见文字（按形状顺序，约等于阅读顺序）
unzip -p "$OUT" ppt/slides/slide3.xml | grep -o '<a:t>[^<]*</a:t>' | sed 's/<[^>]*>//g'

# 第 N 页的演讲者备注（第一个 <a:t> 是页码占位，可忽略）
unzip -p "$OUT" ppt/notesSlides/notesSlide1.xml | grep -o '<a:t>[^<]*</a:t>' | sed 's/<[^>]*>//g'
```

逐条核对：

- 大纲的每一节都有对应页面，没有漏页、没有重复页；
- 没有占位符 / 模板话术（`xxx`、`lorem`、`TODO`、`待补`、`[插入…]`、`示例文本`）；
- 可见文字语言一致（标题、正文、页脚都用入口提示指定的输出语言）；
- 要点是短句/短语，一眼扫完；没有把大纲段落原样搬上页面；
- `> 备注：` 的内容出现在 notes 里，**没有**出现在页面文字里；
- 表格 / 图表 / 大数字与大纲数据一致，没有编造。

> 反例：页面文字里出现 "备注" / "备注：" 字样 → 说明备注被画到页面上了，必须改。

## 2. 版式审计（逐页重算，禁止凭感觉）

对**每一页**把下面的量重新列一遍（可以心里过，但必须过）：

- `contentLeft` / `contentRight` / `contentWidth` / `contentBottom`、`cursorY` 的推进序列；
- 每个元素的 `x / y / w / h` 是否由这些常量推导（而非抄来的数字）。

逐条判定：

1. `x ≥ contentLeft`、`x + w ≤ contentRight`、`y ≥ 0`、`y + h ≤ contentBottom`；
2. 页脚带里**只有**页码（可加一行极短来源）；
3. 子元素完全落在父容器 `innerTop ~ innerBottom` 之间，**没有**用整段父高当子可用高度；
4. 内容框互不重叠（底衬叠字除外），卡片间距一致；
5. `slides-layout` 里的 6 类反模式逐条对照（先画后算 / 魔法坐标 / 嵌套双重计价 / 满高卡+底 strip / 百分比互撞 / 标题吃首卡）；
6. 循环里坐标确实随索引递增；
7. 文本量 vs 容器：`type.body` 的正文放在这个宽度里会不会换行溢出（会 → 减字或拆页）；
8. 字号层级没有塌陷（页标题显著大于正文），正文/注释没有加粗。

## 3. 主题 token 审计（唯一来源）

- `compile.js` 是否 `require` 了 `../skills/theme/<visual_style>/theme.json`，并把 `tokens.palette` 作为 `theme` 传给每页 `createSlide(pres, theme, tokens)`；
- 页面代码里**不应出现任何写死的 6 位 hex**（除 `"FFFFFF"` 这类必要反白）——颜色只能来自 `theme.*` / `tokens.chart.*`；
- 字号只来自 `tokens.type.*`，边距/间距只来自 `tokens.space.*`（乘 `PAGE_*`），圆角只来自 `tokens.radius.*`；**没有**自造的字号表/间距表/第四套配色；
- 全篇只用了当前 `visual_style` 那一套：没有混入其它风格的色值或风格（如 cute 里出现直角报表风）。

## 4. 文件审计

- `slides/slide-01.js … slide-0N.js` **连续无缺号**，且 N == `compile.js` 里的 `totalSlides`；
- **一页一个文件**：没有把多页合并进一个文件、没有「页面数据数组 + 循环 `addSlide()`」的批量写法；每个 `slide-NN.js` ≤ ~120 行 / ~8 KB；
- 每个页文件都 `module.exports = { createSlide, slideConfig }`，`createSlide` **不是** async；
- 引库路径是 `../vendor/standalone.cjs`（禁止 `require("pptxgenjs")`）；
- `await pres.writeFile({ fileName })`；
- `node <workspace>/slides/compile.js` 退出码 0（失败就看 stderr **定位到具体页**，只重写/`EditFile` 那一页，不要整份重写）；
- 最后调 `CheckPPTX <输出路径>`，返回 OK。

## 5. 修复与止损

- 硬伤（重叠、溢出、越界、备注丢失、占位符、语言混用）**必修**：回到 `slides-layout` 的 ①②③ 用游标重算，改对应 `.js` 后再编译、再自检。
- 优先**减项 / 删字 / 拆页**；禁止压 `gap`、禁止缩到看不清、禁止把内容画进页脚带。
- `CheckPPTX` 通过 + 自检无硬伤 → **立即结束**。不要为"再好看一点"反复整页重写，也不要再 `ls` / `find` / 通读文件。
