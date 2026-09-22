---
name: animation
description: 镜内动画配方与手艺：转场、入场、三段结构、机制动效配方、环境动效纪律。写任何 GSAP 动画前读。
---

# 动画系统

本技能是 `video-generate` 入口提示的知识库（位于工作目录 `skills/animation/SKILL.md`）。
**写任何镜内动画前读本技能**。时间为**子 composition 局部时间**（从 0 起）；契约细节见 `skills/hyperframes-contract/SKILL.md`。

## 场景转场

- 转场 = 上一镜退场与下一镜入场**同时发生**；禁止「先淡出再入场」。
- 按「时间轴重建」的转场规则执行：`cut` 硬切；`crossfade` 槽位重叠 0.4s；`wipe`/`slide` 重叠 0.5s。
- 转场写在**根**时间线上，操作 host 槽位（`#el-shot-NN`）或其包装；不要进子文件改兄弟镜。
- 最后镜头允许在根时间线做退场（淡出到结束定格）。

## 入场规则

- 子 composition 内：`tl.fromTo(el, { from }, { to, duration, ease }, 0.1~0.3)`（局部时间）。
- 缓动：入场用 `.out`（`power3.out` / `expo.out` / `power4.out`），退场用 `.in`，位移之间 `.inOut`。
- 方向要变化：不要所有元素都 `y:30, opacity:0`；交替从左、从右、缩放、纯透明度、字距展开。
- 错峰：一组元素 `items × stagger ≤ 0.5s`；按重要性排序。
- 速度有对比：快 0.15–0.3s / 中 0.3–0.5s / 慢 0.5–0.8s。
- **T 型分工**：标题与内容卡入场方向错开（标题自上落下、卡自两翼入），入场方向即版式分工，天然不撞。
- **入场即层级（Motion is typography）**：主焦点用快 / 重的入场（短距离、`expo.out`），从属用轻 / 慢——入场方式携带层级信息，防全屏同速乱入。

## 每镜三段（默认都要有）

| 阶段 | 占镜头时长 | 写什么 |
| --- | --- | --- |
| Build 入场 | 0–25% | 元素错峰入场；第一个动画从局部 `0.1~0.3s` 起 |
| Breathe 发展 | 25–75% | **按分镜 `motion` 发展段执行**：听到哪句 → 变到哪；这是成片灵魂 |
| Resolve 收束 | 75–100% | 焦点钉死或结论态停稳；除转场外不做退场 |

分镜的 `visual_demo` 必须落实为本镜中段的机制动画（路径生长 / 翻转 / 填充 / 对撞 / 数字跳动），对齐具体口播。

## 环境装饰

- 每镜 2–5 个装饰元素（背景光晕 / 网格 / 细线 / 色块）共享一种缓慢动效。
- 幅度克制：缩放 ±0.01–0.03，位移 2–6px，周期 2.5–4s；光晕峰值透明度 ≤ 0.45。
- 多个元素同时 idle 时幅度按 `1/√N` 递减，并错开周期。
- 顺序：先做口播驱动的序列揭示 → 再轻微 jitter → 呼吸是最后手段；宁可不动也别乱动。
- `IDLE_START ≥ 入场落定 + 0.1s`；用 `ease:"none"` 的相位 proxy，从 `sin(0)=0` 起，**叠加**到入场终态（不要另起 `fromTo` 覆盖入场结果）。
- 长镜（>6s 或 >30% 镜长）：幅度减半、周期放到 3–4s，并加 **settle-and-fade 尾巴**（最后 ~20% 把包络降到 0），让转场前画面停稳。
- **禁** CSS `@keyframes` 做 idle（浏览器时钟与 seek 时钟不同步）。
- **所有装饰动效有限且在本镜窗口内收尾**（最后一段循环结束 ≤ 本镜时长 − 0.1s）——转场发生时背景必须停稳，不许抢戏。

## 动画配方库（可直接复制；时间为**子 composition 局部时间**，从 0 起）

**A. 通用入场（标题 + 副标 + 分隔线）**

```js
const t0 = 0.15;
tl.fromTo("#shot-01-title", { y: 70, opacity: 0 }, { y: 0, opacity: 1, duration: 0.6, ease: "power3.out" }, t0);
tl.fromTo("#shot-01-sub", { x: -60, opacity: 0 }, { x: 0, opacity: 1, duration: 0.5, ease: "expo.out" }, t0 + 0.12);
tl.fromTo("#shot-01-rule", { scaleX: 0 }, { scaleX: 1, duration: 0.5, ease: "power2.inOut", transformOrigin: "left center" }, t0 + 0.22);
```

**B. 瀑布式标题（逐词从下方砸入）**

```html
<h1><span class="w">自动</span> <span class="w">生成</span> <span class="w">视频</span></h1>
<style>#shot-01 .w { display: inline-block; opacity: 0; }</style>
```

```js
let tw = 0.15;
document.querySelectorAll("#shot-01-title .w").forEach((el, i) => {
  const heavy = i === 0;
  const y = heavy ? 70 : 44;
  const dur = heavy ? 0.18 : 0.14;
  tl.set(el, { opacity: 1, y }, tw);
  tl.to(el, { y: 0, duration: dur, ease: "power4.out" }, tw);
  tw += dur - 1 / 60;
});
```

**C. 平滑缩放入场（卡片/图标，无回弹）**

```js
tl.fromTo("#shot-01-card", { scale: 0.6, opacity: 0, y: 24 }, { scale: 1, opacity: 1, y: 0, duration: 0.6, ease: "power3.out" }, 0.25);
```

**D. 数字滚动（计数器）**

```html
<span id="shot-02-counter" style="font-variant-numeric: tabular-nums; display: inline-block">0</span>
```

```js
const counterEl = document.getElementById("shot-02-counter");
const st = { v: 0 };
tl.to(st, { v: 12800, duration: 1.6, ease: "power3.out", onUpdate: () => { counterEl.textContent = Math.round(st.v).toLocaleString(); } }, 0.4);
tl.fromTo(counterEl, { scale: 0.5 }, { scale: 1, duration: 1.6, ease: "power3.out" }, 0.4);
```

**E. SVG 描边（路径/图标绘制）**

```js
document.querySelectorAll("#shot-03 .draw path").forEach((p) => {
  const len = p.getTotalLength();
  p.style.strokeDasharray = `${len}`;
  p.style.strokeDashoffset = `${len}`;
});
tl.to("#shot-03 .draw #p1", { strokeDashoffset: 0, duration: 0.6, ease: "power2.out" }, 0.3);
```

**F. 光晕绽放 + 有限呼吸**

```js
tl.fromTo("#shot-03-glow", { opacity: 0, scale: 0.85 }, { opacity: 0.28, scale: 1, duration: 1.0, ease: "power2.out" }, 0.1);
const ph = { p: 0 };
const glowEl = document.getElementById("shot-03-glow");
tl.to(ph, {
  p: Math.PI * 2 * 2, duration: 4.0, ease: "none",
  onUpdate: () => {
    const s = Math.sin(ph.p);
    glowEl.style.opacity = String(0.28 + s * 0.04);
    glowEl.style.transform = `scale(${1 + s * 0.02})`;
  },
}, 1.1);
```

**G. 数据条 / 进度条**

```css
#shot-04 .bar { transform-origin: left center; }
```

```js
tl.fromTo("#shot-04 .bar", { scaleX: 0 }, { scaleX: 1, duration: 0.9, ease: "power3.out" }, 0.4);
```

**H. 打字机（TextPlugin 已包含在 vendor 里）**

```js
gsap.registerPlugin(TextPlugin);
const txt = "让创意自动落地";
tl.to("#shot-05-typed", { text: { value: txt }, duration: txt.length / 8, ease: "none" }, 0.3);
```

**I. 字幕逐条出现**

```js
document.querySelectorAll("#shot-06 .cap").forEach((el, i) => {
  tl.fromTo(el, { opacity: 0, y: 24 }, { opacity: 1, y: 0, duration: 0.35, ease: "power2.out" }, 0.3 + i * 0.4);
});
```

**J. 叠化转场（写在根时间线；上一镜槽位 A → 本镜槽位 B）**

```js
// B 的 data-start = A 窗口结束 - 0.4
const T = aWindowEnd - 0.4;
tl.to("#el-shot-01", { opacity: 0, duration: 0.4, ease: "power1.inOut" }, T);
tl.fromTo("#el-shot-02", { opacity: 0 }, { opacity: 1, duration: 0.4, ease: "power1.inOut" }, T);
```

**K. 推挤转场（根时间线横向滑动槽位）**

```js
const T = aWindowEnd - 0.5; // B 的 data-start = T
tl.to("#el-shot-01", { xPercent: -100, duration: 0.5, ease: "power3.in" }, T);
tl.fromTo("#el-shot-02", { xPercent: 100 }, { xPercent: 0, duration: 0.5, ease: "power3.out" }, T);
```

**L. 末尾淡出（根时间线，仅最后镜头）**

```js
tl.to("#el-shot-Last", { opacity: 0, duration: 0.6, ease: "power1.in" }, TOTAL - 0.6);
```

## 机制动效要点（把分镜的 `visual_demo` 画对）

分镜的机制动词优先按下面的要点实现，**不要只做淡入淡出**：

- **路径生长 / 描边**：setup 时 `getTotalLength()` 量长度，`strokeDasharray = len`、`strokeDashoffset = len`，再 tween 到 0；描边图形 CSS 必须 `fill: none`（否则填充立即出现，毁掉描画）；复杂路径长度不准时略放大 `len × 1.05`；圆环先 `rotate(-90deg)` 让描边从 12 点开始。分段描边每段 `0.3–0.8s`，后一段时长取前一段的 70–80%，`power2.out`（禁 `back`/`elastic`）；先描边后填充（`fillOpacity 0→1`）；虚线流动的偏移取 dash 周期的整数倍。**连接线两端必须落在真实元素上**且承担揭示 / 路由 / 验证 / 强调，只装饰空白的线删掉。
- **图标 / 示意件「活起来」**（4 式）：旋转（`rotate(deg cx cy)`，分针 0.5–2 圈 / 秒针 4–10 圈，别落在整数圈）；摆动（对置两组 `rotate(±sin·amp)`，符号相反）；脉冲（`scale(1+sin·amp)` + opacity，外环相位滞后内点 π/2，振幅 0.05–0.20）；虚线流动（`strokeDashoffset` 线性，偏移取 dash 周期整数倍，负值 L→R）。**坑**：绕指定中心旋转必须用 `el.setAttribute("transform", "rotate(deg cx cy)")`，CSS `transform-origin` + `transform-box: fill-box` 对细线会绕错中心。多部件相位错开（如秒针快于分针、外环滞后内点 π/2）。禁 `requestAnimationFrame`，连续动作用时间线上的线性 proxy。
- **SVG 中心变换（GSAP）**：绕内部点旋转 / 缩放优先用 `svgOrigin`（viewBox 全局坐标）；`svgOrigin` 与 `transformOrigin` 不能同元素并用；构建时间线前先让 SVG 挂载且有尺寸（detached / 0 尺寸时几何解析为 0）。
- **连线网络（avatar cloud）**：一个 `<svg viewBox="0 0 1920 1080">` 覆盖画布，JS 用 `createElementNS` 注入 `<line>` 连接节点对；节点错峰 pop，线段 dash 描画，端点吸附节点边缘；头像环 8–12 个、单体 80–120px、径向 20–30%W × 18–25%H，环须容纳全部且不重叠；连线终止于 hub 边缘（hub z 在线上）；成网后 dwell ≥1s。
- **流程图 / 连接线**：节点 + 连接线；连线端点吸附节点边缘，箭头用 `marker-end`（`refX` / `refY` / `orient="auto"` 设对），别让箭头脱离线。
- **数字 / 数据**：数字与 scale 共用同一 timeline 位置；柱用 `scaleY`、进度条用 `scaleX` 从 0 生长；数字用 `tabular-nums` + `Math.round`。
- **数据默认单焦点**：数字英雄居中 OR 左数右图，二选一；分屏只在分镜点名时用，别在一镜里混两种统计版式。
- **对撞 / 替换**：入场元素驱动退场元素（同一 timeline 位置的三条并发 tween），退场元素时长取入场元素的 40–50%。
- **整组推挤 / 位移**：慢-快-慢三段：入段 `power3.in`（10% 距离 / 20% 时间）→ 爆发 linear（65% / 18%）→ 尾段 `power4.out`（25% / 62%）；尾段**时间** ≥ 3× 入段（不够就延长尾段时间，不是距离）；在爆发段揭示新内容（爆发会遮住出现）。
- **下划线 / 圈画 / 涂高**：用 `scaleX` 从 0 生长的规则线，或 SVG 描边；强调笔触跟随关键词，不整句乱画。
- **粒子 / 爆点**：用下标推导的确定性散布（禁 `Math.random`），粒子数 ≤ 40，单条 `ease:"none"` 的 proxy tween 在 `onUpdate` 里按弹道公式算位置。
- **环境呼吸 / 光晕**：有限 repeat 的 `sine.inOut` yoyo，或在 proxy 的 `onUpdate` 里读 `tl.time()` 计算；光晕峰值透明度 ≤ 0.45。
- **跨镜连续感**：退场用 `.in` + 轻微 blur，入场用 `.out` 从 blur 恢复，让 cut 处速度匹配。

## 缓动与弹簧（别全场一个 ease）

- 默认 `{ duration: 0.6, ease: "power3.out" }`；**平滑优先于弹跳**：`back` / `elastic` 是稀有语气，全片 ≤2 处，只有 `cute` 风格允许明显回弹。
- 同一镜里同一 ease 的独立 tween ≤2 条；速度要有对比：最慢的动效至少比最快慢 2–3×（补上 0.8–2.0s 的「电影级」档）。
- 想要「活但不弹」：用闭式阻尼弹簧（progress 的纯函数，seek-safe）。参数 `response` 0.25–0.35 → 0.37–0.51s（干脆）/ 0.35–0.50 → 0.51–0.74s（标准）/ 0.50–0.70 → 0.74–1.03s（厚重）；`dampingFraction` 0.80–0.85 ≈ iOS 感（1–1.5% 过冲）、0.60–0.70 明显活泼（5–10%）。**禁**有状态弹簧积分器（不可 seek）；没有弹簧函数时用 `power3.out`（≈ζ=1）。
- ζ<1 时只让 transform 过冲；opacity 单独走 `power2.out`，否则整块会闪。

## 入场权重与重叠（瀑布式）

- **重叠不排队**：下一个元素在上一元素结束前开始；间隙递减，最后一个 snap 到位；整组 `items × stagger ≤ 0.5s`。
- 权重表：锚点 / 重词 `Y 60–80px`、`0.16–0.20s`；普通词 `Y 40–50px`、`0.13–0.16s`；轻词 / 标点 `Y 30–48px`、`0.10–0.13s`。
- ease 用 `power4.out`（更 snap 用 `expo.out`）；**禁**入场用 `.inOut`。
- **透明度是二值的**：元素初始 `opacity: 0`，用 `tl.set` 在入场点切成 1 再 tween 位移，不要用 fade 到达（fade 会削弱砸入感）。

## 层计划（多元素防遮挡）

- 每个可见元素在 `<style>` 里给**静态** `z-index`（不进 GSAP；`gsap_non_transform_motion` 只查动画属性，静态 z 放行）。
- 分层带：背景装饰 `0-9` / 底层卡片 `10-19` / 主体内容 `20-29` / 强调与标签 `30-49` / 覆盖层 `50+`。
- 同 z 下渲染按 DOM 顺序绘制：**后出现的元素没给 z 就默认盖住先出现的**——这是浏览器默认行为，不是 bug；要改变就得显式分层（官方契约：画面层级用 CSS `z-index` 控制）。
- **先出现即视觉主角**（choreography is hierarchy）：先出现的元素若要持续可见，其 z 高于后出现的元素；后到的元素想压住先来的，必须刻意拨高 z（`reactive-displacement` 里 intruder 显式 z 在上，不然像穿透），否则错开放置或给低层。
- 元素空间重叠时自问三句：是不是故意的？谁在上？被盖的是不是关键文字（是→改布局或降 z；装饰→`data-layout-ignore`；刻意文字叠放→`data-layout-allow-overlap` 标在参与叠放的文字块上）。

## 装配与拥挤防治（多元素共存）

- **密阵直入槽位**：8+ 项的阵列（Logo 墙 / 特征墙 / 收益列表）禁止共享中心爆发入场——各自短程直入最终槽位，装配动画与最终布局解耦，密集阵列永远清楚。
- **焦点槽 + 按位置渐暗**：长列表 / 轮换永远只有一个「亮槽」：新行弹入高亮焦点槽，离槽的邻行按位置降透明度 / 缩小，一步一拍——列表再长眼睛也有落点。
- **骨架 → 真数据**：先把灰色骨架条逐行填充（左→右、带头尖），完成瞬间换成真实数值 / 头像 chip——「数据正在发生」比空等数字专业。

## 内容驱动序列（列表 / 多句口播）

- 每条时长按内容算：`dur = BASE(0.6–1.5s) + 文本长度 × SEC_PER_CHAR(0.03–0.06) + hold`；`HOLD_MID 0.5–1.0s`、`HOLD_FINAL 1.0–2.0s`。
- 一个 `ease:"none"` 的 driver 走总时长，在 `onUpdate` 里反向查找当前条目；**只在条目切换时改 DOM**（逐帧写 `textContent` 会闪）。
- 容器加 `min-height` 预留高度，否则下面的元素会随文字高度抖动。
- 列举 / 连发可用加速节奏：`HOLDS[i] = max(0.15–0.3, 起始 0.8–1.2 × 0.75–0.88^i)`，最后一条用 `HOLD_FINAL`；低于 0.15s 会频闪。

## 关键词标记五式（文字镜最便宜的提升）

- **highlight**：色条 `inset: 0 -6px`、accent 35% 透明度、`scaleX 0→1`（origin 左）、`0.5s power2.out`、圆角 3px；多行 stagger 0.3s。
- **circle 圈画**：环 `130% 宽 × 160% 高`，`translate(-50%,-50%) rotate(-3deg) scale(0)→1`，`0.6s back.out(1.7)`（仅 cute；其余 `power3.out`），3px 描边。
- **burst 爆发线**：~12 条 30° 步进、3px 宽、长度 40–80px，`scaleY 0→1` + opacity，`0.4s power2.out`，stagger 0.03s。
- **scribble 涂鸦**：SVG `Q` 路径，`getTotalLength()` dash 描画，`0.8s power1.inOut`。
- **sketchout 草图框**：两条 2px 线 `∓12°`，`scaleX 0→1`，前后相差 0.15s，`0.3s power2.out`。
- 每 2–3 组用高能量笔触、3–4 组中、4–5 组低，轮换别重复。

## 概念替换（对比 / 换词 / 状态切换）

- **scale-swap**：两个元素同中心、绝对定位同一容器、`transform-origin: 50% 50%`；出场 `scale 1→0.6–0.8` + fade `0.3–0.5s power2.in`，入场 `scale 0.6–0.8→1` + fade `0.45–0.7s`（cute 用 `back.out(1.4–2.2)`，其余 `power3.out`）；重叠 0.1–0.2s；入场 z-index 在上；不要 `display:none` 出场元素；落定后 ≥1s。
- **reactive-displacement 对撞**：**一个** driver tween（`0.6–1.4s`），在 `onUpdate` 里同时算 intruder 与 victim（拆成独立 tween 会失去因果感）；victim 在 driver 的 40–50% 处完成；同轴反向；intruder 带 5–15° 倾斜入场后回正；intruder z-index 在上；落地后 ≥1s。

## 节奏文字（钩子 / 结论）

- 共享一个 `BEATS[]` 节拍数组，句子与节奏装饰都读它；每句入场动作不同（scale+blur 砸入 / 侧向 snap / 上升旋转），全片 ≥3 种 ease。
- 入场 0.35–0.6s；退场 ≤0.25s；BEATS 间隔 1.2–1.8s（<0.8 太赶，>2.5 掉拍）；显示字号 150px+（中文用 120–200px + 字重 / 字距补气势）。
- 重复用 `Math.max(0, Math.floor(holdDur / cycle) - 1)`；`Math.ceil` 会越过 `data-duration` 触发 lint。
- **一屏一事、旧内容零残留**：每个全屏节拍独占画面，到达即清；换词用瞬时硬切（无滚动 / 模糊），前一句不得残留、不得堆叠。

## 动画自检

- [ ] 每个镜头子文件都有入场动画；`visual_demo` 的机制动画已落实在讲解中段；
- [ ] 子文件动画结束时间 ≤ 本镜 `data-duration`（末镜退场除外，写在根上）；
- [ ] 无裸 `gsap.to()` / `gsap.from()`；无 `repeat: -1`；
- [ ] 无并发控制同一元素同一属性；每个装饰元素都有克制动效；
- [ ] 所有可见元素有显式 `z-index`；先出现的元素未被后出现的盖住关键文字；
- [ ] 装饰动效都在本镜窗口内收尾，转场时背景停稳；
- [ ] 每个 `compositions/shot-NN.html` 已单独 check 通过。
