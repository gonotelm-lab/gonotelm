---
name: camera
description: 讲解镜头的镜头语言：2D 相机推拉摇移与微漂移、坐标目标推镜、punch-in/Ken Burns 快捷配方、景深拉焦、速度模糊、3D 飞行。需要给画面加纵深、引导视线或强调正在讲的对象时读。
---

# 镜头语言

本技能是 `video-generate` 入口提示的知识库（位于工作目录 `skills/camera/SKILL.md`）。
相机只改变观看方式，不改变画面内容；分镜没有要求时优先用「快捷配方」的轻微推拉，不要每镜都上相机。

## 通用纪律

- **每镜最多一个相机**；相机 wrapper 包住本镜全部内容。
- 背景放 `.scene`，不要放相机层（相机缩放时背景跟着动会穿帮）；`.scene { overflow: hidden }`。
- 相机 transform 由**唯一** writer 统一写（一个 `onUpdate`，或一组共享 position / duration / ease 的 tween）；`transform-origin: 50% 50%`；`will-change: transform`。
- 相机只用 `power2` / `power3` 系；**禁** `back` / `elastic`（镜头过冲会晕）。
- 承载信息的文字缩放保持在 **0.95–1.15**；相机留给钩子 / 收束镜，Resolve 段画面停稳。
- 入场 tween 与相机 tween 分开：入场作用在子元素上，相机作用在 wrapper 上，别在同一元素上叠两个 transform。
- **相机克制（全片预算，编排层把关）**：zoom-out（拉远揭示）全片只允许 1 次；单镜内相机真移动 ≤2 次，结束态静止；转场 / 流式输出时段锁帧。运动混乱 90% 来自相机乱动——分镜没点名就只用快捷配方的轻微推拉。

## 2D 相机（推 / 拉 / 摇 / 移）

单个 `cam = { scale, x, y }` 状态对象，一个 writer 合成 `translate(x,y) scale(S)`，构建后先 `applyCamera()` 播种第 0 帧。

- 意图与位移相反：画面右移 = world `x` 为负；推近 = world `scale > 1`。
- 单层反解：`T = -offset × S`（单 wrapper 公式，别和下面双 wrapper 的 `T = -offset` 混用）。
- 幅度：`1.02–1.05` 微妙；`1.05–1.15` 强调；`1.15–1.30` 聚焦区域；`1.5–2.5` 戏剧；`<5%` 看不出来，`>30%` 电影感。
- 时机：内容落位后 ~0.5s 起推；`ZOOM_DUR 1.0–2.0s`（<0.8 瞬移、>2.5 拖沓）；推完 `DWELL ≥1.0s`。

## 坐标目标推镜（推到你正在讲的那个节点）

双 wrapper：`.zoom-outer` 负责 scale（origin 50% 50%），`.zoom-inner` 负责 translate；两个 transform 不要放同一元素。

- 反解：`T = -offset`（与 scale 无关；`T = -offset × (S−1)` 是常见错误直觉）。
- 布局落位后（字体就绪）测一次目标中心，bake 成常量：`offsetX = r.left + r.width/2 - W/2`；**不要**在 tween 里逐帧测量。
- 留白预算：`maxScale = Math.min(0.88*W/r.width, 0.88*H/r.height)`，峰值目标 ≤88% 画布（97%+ 读成裁切）。
- scale 与 counter-translate **共享 position、duration、ease**，否则目标会漂。
- `ZOOM_SCALE 1.5–3×`；`ZOOM_DUR 1.0–2.0s`；`DWELL ≥1.0s`。

## 多段相机（有呼吸的推拉）

一条相机 wrapper，相位代理走 2–3 段，微漂移叠加在同一个 writer 里：

- 相位：focus-in（back → neutral → 轻推）/ dramatic reveal（push → neutral → pull）/ steady push / bookend pull。
- 比例：P1 `0.88–0.96`、P2 `0.98–1.02`、P3 `1.04–1.15`；P2 `0.3–1.0s` 起 / `1.0–1.8s`；P3 `2.0–4.0s` 起 / `1.0–2.0s`；ease `power2/3.out` 或 `power2.inOut`。
- 微漂移：`DRIFT_CYCLES 1–3`，`AMP_X 2–8px`、`AMP_Y 1–4px`，`FREQ_RATIO ≈1.3`（=1.0 会变成机械斜线）；漂移 tween 铺满本镜时长。
- 相位触发对齐内容节拍，不是对表；漂移 >8px 会读成抖动。

## 快捷配方（性价比最高，优先用）

作用于**内层 wrapper**，永远不碰 `.clip` 元素：

```js
// 轻微推近（Ken Burns）：整镜缓慢放大
tl.fromTo("#shot-03-cam", { scale: 1.05, xPercent: 0 }, { scale: 1.2, xPercent: -12, duration: 4, ease: "none" }, 0);
// 强调 punch-in：快进快出，配重音
tl.to("#shot-03-cam", { scale: 1.35, xPercent: -8, duration: 0.18, ease: "power2.in" }, 1.2);
// 裁切 / 重构图：只改取景
tl.to("#shot-03-cam", { clipPath: "inset(8% 12% 6% 10%)", xPercent: -4, duration: 1, ease: "power2.inOut" }, 2.0);
```

- 位移类相机作用在大标题 / 图形上；小正文别做 `xPercent` 横移（影响阅读）。
- 结束态必须仍然可读；punch-in 后记得留出观看时间。

## 景深 / 拉焦（视线引导）

每层一个 CSS 变量：`--dof: 0px; filter: blur(var(--dof)); will-change: filter;`，在时间线上 tween `--dof`（paint-only，seek-safe）。

- `BLUR_PER_DEPTH 3–6px/层`；`MAX_BLUR`：soft 8 / default 16 / heavy 24；`GRID_BLUR 6–12px`；`DIM_LEVEL` 0.4 强 / **0.55 默认** / 0.7 弱（不要 <0.35）；`FOCUS_DUR 0.5–1.2s`。
- 拉焦：先 `tl.set` 把 B 层预模糊，再两条共享 start+duration 的 tween（A→虚化+压暗，B→清晰+复原）在中点交叉。
- 聚焦层 `z-index` 在虚化层之上；模糊小 / 成组层，不要糊整屏大背景（代价 = 半径 × 像素面积）；中文正文 <28px 不要糊。
- **尾巴先归零**（`--dof: 0`）再交接转场，半虚化交接会像渲染故障。
- 景深与相机独立：相机 transform wrapper，景深 blur 叶子层。

## 速度模糊（可选，只用于穿推 / 甩入）

SVG `feGaussianBlur` 的 `stdDeviation` 用 proxy tween 在 `onUpdate` 里 `setAttribute`；模糊包络与位移**共享 window 与 ease**，峰值在速度最大处，落定归 0。

- `PEAK_BLUR 8–30`（默认 18；wrapper 上限 18–20）；`MOVE_DUR 0.25–0.6s`；filter 区域 `x/y=-50%`、`width/height=200%`。
- 只用于入场 / 穿推，落定后 ≥1s 清晰；不要用于镜中退场。

## 3D 飞行（可选，仅 hero 镜）

静态 `.stage { perspective: 700–1400px }` + `.world { transform-style: preserve-3d }`；`cam = {x,y,z,rx,ry}` 固定顺序 `translate3d(x,y,z) rotateX(rx) rotateY(ry)`（平移在旋转外，x/y/z 才是屏幕对齐的）。

- `|rx| ≤ 65°`、`|ry| ≤ 30°`；`z + 元素Z ≤ 0.6 × perspective`；dive `0.6–1.0s power4.out`、flatten `1.2–2.0s power2.inOut`。
- `.world` 禁 `filter` / `opacity<1` / `overflow` / `clip-path` / `mask`（任一都会压平 3D）；模糊放 `.stage`，景深放叶子卡片，背景放 `.scene`。
- 只在落点读字：落点近水平并 hold ≥1s；`data-layout-allow-overflow` 加在 `.world`。

## 自检

- [ ] 每镜最多一个相机，且相机与入场 tween 不在同一元素；
- [ ] 相机 wrapper 与背景分离，`.scene` 有 `overflow:hidden`；
- [ ] 文字缩放峰值 ≤1.15；Resolve 段画面停稳；
- [ ] 景深在转场前已归零；中文正文没有被糊；
- [ ] 相机位移没有把内容推出画框。
