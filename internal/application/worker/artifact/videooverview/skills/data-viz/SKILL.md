---
name: data-viz
description: 数据与信息图动效：增长条、进度填充/环、星级、数字滚动与图形同步、图表读头。画数据镜、信息图或需要数字+图形同拍时读。
---

# 数据与信息图

本技能是 `video-generate` 入口提示的知识库（位于工作目录 `skills/data-viz/SKILL.md`）。
数据镜 = 「一个数字 + 一个视觉元素」的同拍，不是孤零零的数字。数据镜是全片允许的「密集例外」。

## 规则

- 每个数字配一个视觉元素（填充条 / 进度环 / 形状）；同概念的连续数据保持同一视觉空间，只变值。
- 禁饼图、多轴图、6 格以上仪表盘、网格线 / 刻度 / 图例。
- 单焦点居中 vs 分屏二选一，全片一致；强调色每镜 ≤1 个。
- 图形一律用 transform 生长（`scaleX` / `scaleY` / dash），不 tween `width` / `height`。

## 增长条

终态高度写在 CSS；只 tween `scaleY: 0 → 1`，`transform-origin: bottom center`。

```js
tl.fromTo("#shot-04 .bar", { scaleY: 0 }, { scaleY: 1, duration: 0.7, ease: "power3.out", stagger: 0.08 }, 0.4);
```

- 4–6 条；最后 / 当前条用 `accent`，其余用 `primary` 明度档；柱内**禁渐变**（柱越高顶部越浅会反向误导对比）。

## 进度填充

- `.fill` **必须 `width: 100%`**（没有宽度时 `scaleX(0)` 仍是 0，条会隐形，自动检查也发现不了）；`transform-origin: left center`。
- `tl.to(".fill", { scaleX: PCT, duration: 1.0, ease: "power2.out" }, t)`，`PCT` 是 0–1 的比例。

## 进度环

- setup 时 `getTotalLength()`；`strokeDashoffset = LEN × (1 − pct)`；圆环先 `rotate(-90deg)`，让填充从 12 点开始。

## 星级评分

- 灰底 + 高亮副本：`clip-path: inset(0 100% 0 0) → inset(0 (100−pct)% 0 0)`，`1.0s power2.out`。

## 数字滚动（与图形同拍）

```js
const counterEl = document.getElementById("shot-04-counter");
const st = { v: 0 };
tl.to(st, { v: 12800, duration: 1.6, ease: "power3.out", onUpdate: () => { counterEl.textContent = Math.round(st.v).toLocaleString(); } }, 0.4);
tl.fromTo(counterEl, { scale: 0.5 }, { scale: 1, duration: 1.6, ease: "power3.out" }, 0.4);
```

- 数字与图形**同一 timeline 位置、同一 ease**（一个和弦，不是琶音）；多指标同起同收。
- `font-variant-numeric: tabular-nums` + 固定宽容器（防跳动）；`COUNT_DUR 1.2–2.5s`（<0.8s 像闪一下）。
- `START_SCALE` 取终字号的 40–60%；`Math.round`（不是 floor）；**禁** `back` / `elastic`；不要在 `onUpdate` 里改字号。
- 后缀（% / × / +）在计数落地后 `0.3–0.6s` 进；标签在 `COUNT_DUR/2` 前到。

## 图表读头（让已画好的图表动起来）

- 数据是 setup 时的字面量；点坐标由纯函数 `X(i)` / `Y(v)` 一次算好；**一个** driver `p: 0→1`，在 `onUpdate` 里推 tracking line + marker + tooltip，全部是 `p` 的纯函数。
- marker 的 `y` 在两个相邻烘焙点之间插值；tooltip 只在最近数据点索引变化时改文本（`lastIdx` guard），加 `tabular-nums` 和固定 `min-width`。
- 构建后先用 `p = 0` 播种一次，否则 seek 到 t=0 会看到 HTML 默认位置。
- SVG `viewBox` 用像素坐标（`viewBox="0 0 W H"` + 同尺寸），与 HTML tooltip 共用一套坐标。
- `N 10–40` 点；`SCRUB_DUR 1.5–3s`；`power1.inOut`（峰值停用 `power3.out`）；**禁** `back.out`；末值 hold ≥0.8s；图先画完（描边 / 生长）再 scrub。

## 自检

- [ ] 每个数字都有配套视觉元素，且与数字同拍；
- [ ] 增长条 origin 在底部、填充条有 `width:100%`、环从 12 点起画；
- [ ] 数字用 `tabular-nums` 且容器固定宽；
- [ ] 无饼图 / 网格线 / 图例 / 柱内渐变；
- [ ] 图表读头的 tooltip 只在索引变化时更新文本。
