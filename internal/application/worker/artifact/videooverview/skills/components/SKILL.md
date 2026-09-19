---
name: components
description: 使用工作目录 components/ 里的现成动画片段：怎么挑、怎么把 HTML/CSS/JS 并进宿主 composition、怎么接时间线。需要现成效果（图表 / UI mockup / 转场 / 手绘 / 氛围等）时读。
---

# 组件库（Components）

工作目录 `components/` 有现成动画片段（**只读**），索引在 `components/INDEX.md`。
组件是**片段**，并进宿主 composition，不单独成镜；本技能讲怎么挑、怎么并入。

## 挑选

- 先读 `components/INDEX.md`（按用途分组），按需要找名字，再读 `components/<name>/<name>.html`。
- 每个组件文件顶部有注释头：用途、可调值、依赖；**先读注释头再用**。
- 没有合适的就按 `skills/animation/SKILL.md` 自己写。组件是加速手段，不是必须。

## 并入宿主（四步）

1. **HTML**：把组件的 HTML 元素放进宿主 composition 的内容层（`<div data-composition-id="...">` 内），放到合适的 z-index。
2. **CSS**：把组件的 `<style>` 规则并进宿主的 `<style>`。
3. **JS 初始化**：把组件的 `<script>` 初始化代码并进宿主的 `<script>`，放在时间线代码**之前**。
4. **时间线**：若组件暴露 GSAP 调用（注释头会写），把对应 tween 加进宿主的 `tl`，按本镜节奏调 start / duration / ease。

## 关键约束

- 组件**没有自己的时间线**，参与宿主 composition 的时间轴；尺寸 / 时长继承宿主。
- 宿主已加载本地 GSAP（子文件也用根相对 `vendor/gsap.browser.js`，**不要** `../`）；组件里**没有** CDN 引用，也**不要**补任何外链。
- 文案 / 颜色 / 尺寸按本片语言与五色盘替换；不要照搬示例文案。
- 同一镜同类效果不要叠加；一个组件通常只用一次。
- 并入后跑 `hyperframes check`；报错按 `skills/hyperframes-contract/SKILL.md` 处理。

## 例：grain-overlay（纯 CSS，无时间线）

组件是绝对定位覆盖层：把它的 div 放进内容层、CSS 并进 `<style>` 即可，不需要 `tl` 调用。

## 例：shimmer-sweep（需要时间线）

HTML 包住目标元素 → CSS 并进 `<style>` → JS 自动注入 `.shimmer-mask` → 在 `tl` 里加 `fromTo` 扫光（`duration` / `stagger` 自定）。

## 自检

- [ ] 组件 HTML / CSS / JS 三段都已并入宿主（没漏 JS 初始化）；
- [ ] 时间线调用进了宿主 `tl`，start / duration 对齐本镜口播；
- [ ] 无外链、无 CDN；文案 / 颜色已按本片替换；
- [ ] 并入后 `check` 通过。
