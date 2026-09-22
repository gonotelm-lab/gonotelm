---
name: hyperframes-contract
description: HyperFrames 合成契约与排错：文件架构、骨架、data 属性、时间线/确定性规则、lint 报错表、失败降级。写任何 composition HTML 前必读；check / render 报错时查阅。
---

# HyperFrames 契约与排错（必读）

本技能是 `video-generate` 入口提示的知识库（位于工作目录 `skills/hyperframes-contract/SKILL.md`）。
**写任何 HTML 前先通读本文件**；`check` / `render` 报错时查文末对照表与失败降级。工作流程、命令、时间轴算法、渲染与压制以入口提示为准。

# HyperFrames 契约（必须遵守）

## 架构：薄 index + 每镜子 composition（默认）

| 文件 | 形态 | 职责 |
| --- | --- | --- |
| `index.html` | **Standalone**（根在 `<body>`，**禁止**包 `<template>`） | 槽位编排、全部旁白音频、近空根时间线、跨镜转场 |
| `compositions/shot-NN.html` | **Sub-composition**（内容必须在 `<template>` 内） | 单镜画面、CSS、局部 paused 时间线 |

校验要求（回答「每个 composition 都要能过检」）：

1. **每个** `compositions/shot-NN.html` 写完后用临时校验目录 `hyperframes lint <临时目录>`（本版本没有 `check -c`）→ **0 error** 才算该镜交付。
2. 拼好 `index.html` 后再跑整项目 `hyperframes check` → **0 error**
   （同时验证跨文件挂载：host `data-composition-id` == 子文件内部 id == `__timelines` 键）。
3. 静态 lint 过不了的跨文件挂载问题，只会在整项目 check / render 时暴露——因此两步都要做。

## 子 composition 骨架（`compositions/shot-01.html`）

`<style>` / `<script>` / 根节点必须在 `<template>` **内部**（runtime 只克隆 template 内容；写在 `<head>` 的样式会被丢掉）。

```html
<!doctype html>
<html lang="zh">
  <head>
    <meta charset="UTF-8" />
    <title>shot-01</title>
  </head>
  <body>
    <template>
      <script src="vendor/gsap.browser.js"></script>
      <style>
        /* 根必须用 #root，禁止用 class 给带 data-composition-id 的根设尺寸（lint: subcomposition_root_styled_by_class） */
        #root {
          position: absolute;
          inset: 0;
          width: 1920px;
          height: 1080px;
          overflow: hidden;
          background: var(--surface);
          font-family: var(--font-display-en), "CJK", sans-serif;
          color: var(--text);
        }
        @font-face {
          font-family: "CJK";
          src: local("Noto Sans CJK SC"), local("Noto Sans SC"), local("Source Han Sans SC"),
            local("WenQuanYi Zen Hei"), local("Droid Sans Fallback"), local("PingFang SC"),
            local("Microsoft YaHei");
        }
        .inner { position: absolute; inset: 0; }
      </style>

      <div
        id="root"
        data-composition-id="shot-01"
        data-width="1920"
        data-height="1080"
        data-duration="2.70"
      >
        <div class="inner" id="shot-01-inner">
          <!-- 背景层 / 主体层 / 前景层；元素 id 一律加场景前缀 shot-01- -->
        </div>
      </div>

      <script>
        const tl = gsap.timeline({ paused: true });
        // 局部时间从 0 起；不要使用全局 shotStart
        // tl.fromTo("#shot-01-title", …, 0.15);
        window.__timelines["shot-01"] = tl;
      </script>
    </template>
  </body>
</html>
```

子文件硬性规则：

- host 槽位 / 内部根 / `__timelines` **三者 id 必须同名**（如都是 `shot-01`）。
- 时间线用**局部时间**（镜头内 0 … shotWindow）；`data-duration` = 本镜 `shotWindow`。
- 元素 id 加前缀（`#shot-01-title`），避免组装后 id 冲突。
- **不要**在子文件里放旁白 `<audio>`（音频全部在根）。
- 根样式用 `#root`，不要给根加会参与选择器的 class 尺寸规则。

## 薄 `index.html` 骨架（编排层）

```html
<!doctype html>
<html lang="zh">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=1920, height=1080" />
    <title>视频</title>
    <script src="vendor/gsap.browser.js"></script>
    <style>
      @font-face {
        font-family: "CJK";
        src: local("Noto Sans CJK SC"), local("Noto Sans SC"), local("Source Han Sans SC"),
          local("WenQuanYi Zen Hei"), local("Droid Sans Fallback"), local("PingFang SC"),
          local("Microsoft YaHei");
      }
      * { margin: 0; padding: 0; box-sizing: border-box; }
      html,
      body {
        width: 1920px;
        height: 1080px;
        overflow: hidden;
        background: #0d1b2a;
        font-family: var(--font-display-en), "CJK", sans-serif;
      }
      #root { position: relative; width: 1920px; height: 1080px; overflow: hidden; }
      /* 子 composition 槽位铺满根 */
      [data-composition-id="main"] > div[data-composition-src] {
        position: absolute;
        inset: 0;
      }
    </style>
  </head>
  <body>
    <div
      id="root"
      data-composition-id="main"
      data-start="0"
      data-width="1920"
      data-height="1080"
      data-fps="24"
      data-duration="TOTAL"
    >
      <div
        id="el-shot-01"
        class="clip"
        data-composition-id="shot-01"
        data-composition-src="compositions/shot-01.html"
        data-start="0"
        data-duration="2.70"
        data-width="1920"
        data-height="1080"
        data-track-index="1"
      ></div>

      <div
        id="el-shot-02"
        class="clip"
        data-composition-id="shot-02"
        data-composition-src="compositions/shot-02.html"
        data-start="2.70"
        data-duration="4.70"
        data-width="1920"
        data-height="1080"
        data-track-index="1"
      ></div>

      <!-- 旁白全部在根；全局时间；每段必须有唯一 id -->
      <audio id="a1" src="audio/audio_0-0.wav" data-start="0" data-duration="2.20" data-track-index="10"></audio>
      <audio id="a2" src="audio/audio_0-1.wav" data-start="2.70" data-duration="1.80" data-track-index="10"></audio>
      <audio id="a3" src="audio/audio_0-2.wav" data-start="4.50" data-duration="2.10" data-track-index="10"></audio>
    </div>

    <script>
      // 根时间线通常近空；只放跨镜转场 / 片尾淡出
      const tl = gsap.timeline({ paused: true });
      window.__timelines["main"] = tl;
    </script>
  </body>
</html>
```

尺寸按分镜 frontmatter 的 `format`（如 `1920x1080`、`1080x1920`、`1080x1080`）设置，
且**三处一致**：viewport meta、CSS 的 `html/body/#root`、`data-width`/`data-height`
（含每个子 composition 与每个 host 槽位）。

常用尺寸对照（分镜未写 `format` 时按内容形态选：讲解/横屏默认 16:9，竖屏短视频默认 9:16）：

| 场景 | 比例 | `data-width` × `data-height` |
| --- | --- | --- |
| 横屏讲解 / 网站 | 16:9 | 1920 × 1080 |
| 竖屏短视频 | 9:16 | 1080 × 1920 |
| 方形 | 1:1 | 1080 × 1080 |
| 竖版海报 | 3:4 | 1080 × 1440 |
| 4K 横屏 | 16:9 | 3840 × 2160 |
| 4K 竖屏 | 9:16 | 2160 × 3840 |

- 渲染时**不要**传 `--resolution`（除非常用预设且与 composition 比例一致）；composition 自身尺寸就是输出尺寸。
- 所有版式用相对/弹性布局（flex、grid、百分比、`inset`），不要硬编码只适合某一分辨率的绝对坐标。

## 根元素属性（`index.html`）

| 属性 | 必填 | 说明 |
| --- | --- | --- |
| `data-composition-id` | 是 | 固定 `main`，必须与 `window.__timelines["main"]` 一致 |
| `data-width` / `data-height` | 是 | 像素尺寸，与 CSS / viewport 一致 |
| `data-duration` | 是 | **TOTAL**，脚本运行前读一次，必须直接写死 |
| `data-start` | 是 | 固定 `0` |

## Host 槽位属性

| 属性 | 必填 | 说明 |
| --- | --- | --- |
| `data-composition-id` | 是 | 与子文件内部 id、`__timelines` 键**同名** |
| `data-composition-src` | 是 | 如 `compositions/shot-01.html` |
| `data-start` / `data-duration` | 是 | 全局时间窗 = 时间轴表的 shotStart / shotWindow |
| `data-width` / `data-height` | 是 | 与根一致 |
| `class="clip"` | 是 | 带 `data-start` 的可视元素必须有 |

## 场景与时间

- 子 composition 内：时间从 **0** 起算；动画结束早于本镜 `data-duration` 0.1–0.3s。
- 根上：槽位 `data-start` 用全局 `shotStart`；旁白 `<audio>` 用全局时间。
- 窗口半开 `[start, start+duration)`：`t = start+duration` 时已隐藏。
- `data-track-index` 只是 Studio 显示轨道；画面层级用 CSS `z-index`。视觉槽位可用同一 track；音频用更高 track（如 10）。

## 音频（决定成片成败）

- **每个 `<audio>` 必须有 `id`**，否则不会被混入，成片静音。
- `data-start` / `data-duration` 必须精确等于时间轴表；同一镜内多段音频背靠背。
- 全部挂在根 `index.html`，不要放进子 composition。
- 不要在代码里调用 `.play()` / `.pause()` / 改 `currentTime`；不要给 `<audio>` 加 `crossorigin`。
- 需要淡出时用**根**时间轴动画（不要改 `data-volume`）：

```js
tl.to("#a2", { volume: 0, duration: 0.4, ease: "power1.in" }, audioEnd - 0.4);
```

## 时间线

- **每个** composition（含每个子文件）只注册**一个** `gsap.timeline({ paused: true })`，在脚本末尾注册。
- 子文件：`window.__timelines["shot-NN"] = tl;`（局部时间）。
- 根：`window.__timelines["main"] = tl;`（通常近空，只做转场）。
- 动画用 `tl.to(...)` / `tl.fromTo(...)`，不要用 `gsap.from`：
  - **`fromTo` 只用来定义「入场初始态」**（该元素在本镜里第一次出现）。元素已经被前序 tween 或 `tl.set` 定过态时一律用 `tl.to(...)`——`fromTo` 的 `from` 会在 immediateRender 时立刻写入，把前序结果盖掉。
  - `from` 与 `to` **不能同值**；只改一个属性、或只想「瞬间置位」，不要为了统一而套 `fromTo`。
- 同一元素、同一属性同一时间只能被一个 tween 控制。
- 所有环境动效必须挂在该 composition 自己的 `tl` 上，禁止裸 `gsap.to()`。
- 不要在 `setTimeout` / `Promise` 里注册时间线。
- 子 composition 时间线**不能**跨文件选中 host 元素；反之根时间线也不要去 tween 子文件内部节点（转场只操作 host 槽位）。

## 渲染保真（避免元素不出现 / 时间线提前结束）

- **timeline sentinel**：子 composition 的时间线末尾加 `tl.set({}, {}, 本镜 data-duration)`，把时间线拉到完整时长，防渲染器在最后一个 tween 之后提前结束、丢结尾帧。
- **opacity 放 wrapper**：框架会把带 `data-start` / `data-duration` 的 timed 元素 opacity 强制为 1；要给 timed 元素做透明度，把 opacity 放在**没有 `data-*` 的 wrapper** 上（直接写在 timed 元素上会被静默覆盖）。
- **full-bleed 底色用独立 clip 层**：不要画在 composition 根上（根会被镜窗门控）；用一条铺满本镜时长的 `class="clip"` 背景层。
- **untimed 全屏层必须自设尺寸**：`position: absolute; inset: 0`，否则会塌成 0 高度、什么都画不出来。
- 根 `data-duration` 是编译期锁定的，脚本改不了；clip 的 `data-duration` 才是活的。
- `data-track-index` 不影响渲染层级（层级由 CSS `z-index` 决定）；重叠的视觉 clip 可以同 track。

## 确定性禁令

- 禁止 `Date.now()`、`performance.now()`、任何真实时钟。
- 禁止未播种的 `Math.random()`；需要随机感用下标推导或正弦相位。
- 禁止渲染时网络请求；禁止 hover / scroll / click 等输入状态。
- 禁止 `repeat: -1`，循环用有限次数：

```js
const repeats = Math.max(0, Math.floor(windowDur / cycle) - 1);
tl.to("#el", { scale: 1.05, duration: cycle, ease: "sine.inOut", yoyo: true, repeat: repeats }, t0);
```

- 禁止对 `.clip` 元素 tween `display` / `visibility`；用内层包装的透明度或零时长 `tl.set`。
- 禁止动画 `width` / `height` / `top` / `left`；用 `scale` / `x` / `y` / `opacity` / `filter` / `clipPath` 替代。
- 禁止 `<br>` 换行正文（短展示标题除外）；用 `max-width` 让文本自然换行。
- 需要 `transform` 的元素必须是 block/inline-block 且有自己的尺寸；CSS 里不要给动画元素写 `transition`。

## 首次 lint 必错项（写的时候就避开）

| 错误 | 规避写法 |
| --- | --- |
| `gsap_css_transform_conflict` | CSS 不写 `transform` 初始值，把初始态放进 `fromTo` 的 from |
| `media_crossorigin_breaks_preview` | `<audio>` 永远不加 `crossorigin` |
| `media_missing_id` | 每个 `<audio>` 都有唯一 id |
| `timed_element_missing_clip_class` | host 槽位带 `data-start` 时加 `class="clip"` |
| `root_composition_missing_duration_source` | 根与子根都写死 `data-duration` |
| `font_family_without_font_face` | 只用 §字体 的内置字体；中文用 `@font-face` 的 `"CJK"` |
| `gsap_repeat_ceil_overshoot` | repeat 用 `Math.max(0, Math.floor(...) - 1)` |
| `missing_gsap_script` | 根与**子文件**都用根相对 `vendor/gsap.browser.js`（子文件**不要**写 `../`） |
| `invalid_parent_traversal_in_asset_path` | 任何资源路径都根相对；`../` 会被 Studio / 直播预览按项目根解析而 404 |
| `duplicate_audio_track` | 重叠音频不要放同一 `data-track-index` |
| `standalone_composition_wrapped_in_template` | `index.html` 根不要包 `<template>` |
| `subcomposition_root_styled_by_class` | 子文件用 `#root` 设尺寸 |

## lint 抓不到、但必须第一稿就避开的写法

以下写法都过 `hyperframes lint`（0 error 0 warning），却会在成片里穿帮，或逼你回头重改。

| 不要写 | 改写成 |
| --- | --- |
| `tl.to(el, { …, duration: 0.01 }, t)` 当「瞬间置位」 | `tl.set(el, { … }, t)`——瞬间改状态**只用 `tl.set`**，不要用趋近 0 的 duration |
| `tl.fromTo(el, { x: 0 }, { x: 76, … }, t)`（from/to 同值，或元素已有前序动画） | `tl.to(el, { x: 76, … }, t)` |
| 用 class 选择器驱动只该动一个 / 一动一组的元素（`tl.to(".sline", …)`） | 每个元素给唯一 `id`，动画一律写 `#id`；同类多元素要一起动就写数组 `["#a", "#b"]` |
| SVG 线条生长用 `scaleX` / `scaleY` | 量 `getTotalLength()` 设 `strokeDasharray` / `strokeDashoffset`，动画 dashoffset |
| 运行时用 JS 改 `textContent` 摆**静态**文案 | 文案直接写进 HTML；只有确实要「变字」时才用 `tl.set` |
| 最后一个 tween 的 `start + duration` 贴住甚至超过本镜 `data-duration` | 先算好本镜槽位秒数，最后一个 tween 收在 `data-duration - 0.1 ~ 0.3s` |

**每写一镜，落盘前对着这张表自检一遍**——比写完再回头 patch 便宜一个数量级。

## 字体

- 可直接使用的内置字体（离线可渲染）：`Inter` `Roboto` `Open Sans` `Lato` `Nunito` `Montserrat`
  `Poppins` `Outfit` `Oswald` `League Gothic` `Archivo Black` `Playfair Display` `EB Garamond`
  `Space Mono` `IBM Plex Mono` `JetBrains Mono` `Source Code Pro` `Noto Sans JP`。
- 别名：`Helvetica Neue`/`Arial` → Inter；`Futura`/`Arial Black` → Montserrat；
  `Bebas Neue` → League Gothic；`Courier New` → JetBrains Mono。
- **League Gothic 与 Archivo Black 只有 400 字重**，不要写 700/900。
- 中文必须落到 `@font-face` 声明的 `"CJK"` 家族；`font-family: var(--font-display-en), "CJK", sans-serif`。
- 数字/数据用 `font-variant-numeric: tabular-nums`。

## 装饰性文字与出血装饰的豁免

- 幽灵字 / 纹理字 / 超低透明度装饰文字加 `data-layout-ignore`，否则对比度审计报假错误。
- 有意出血画框的装饰元素加 `data-layout-allow-overflow`（只加在最小装饰包装上，不要加在正文面板）。
- 单个被裁的主文字用 `data-layout-bleed="true"`（比 `allow-overflow` 精确）。
- **注意** `data-layout-allow-overflow` 会**继承**并静默整个子树的 `text-clipping` / `content-cramped-container` / `foreground-over-panel`；加在过大的包装上会掩盖真实缺陷。
- 叠化期两镜文字重叠报 `content_overlap` 时，给参与叠化的文字块加 `data-layout-allow-overlap`。

# 自检命令（v0.8.35 已验证）

除了 `check` / `lint`，这些文本 / JSON 型命令能在离线沙箱里定位问题（模型可直接读）：

```bash
# 结构化质检：定位选择器 / data 标识 / bbox / 对比度建议色
hyperframes check --json
# 指定关键时刻采样（首帧、中段、叠化缝、结尾前；叠化缝用时间轴表里的 crossfade 时刻）
hyperframes check --at 1.5,4,7.25
# 单镜动画诊断（TARGET 可直接给 composition .html）
hyperframes keyframes --json compositions/shot-01.html
# 媒体（img/svg/video/canvas）超框检测
hyperframes check --frame-check
```

- 可选 `*.motion.json` sidecar（放在**项目目录**，文件名与 composition 同名，如 `index.motion.json`）：`check` 会自动读取并断言。格式：

```json
{
  "version": 1,
  "assertions": [
    { "kind": "appearsBy", "selector": "#shot-01-title", "bySec": 1.2 },
    { "kind": "before", "a": "#shot-01-title", "b": "#shot-01-sub" },
    { "kind": "staysInFrame", "selector": "#shot-01-title" },
    { "kind": "keepsMoving", "withinSelector": "#shot-01-inner", "maxStaticSec": 2 }
  ]
}
```

- 诊断码：`motion_appears_late` / `motion_frozen` / `motion_off_frame` / `motion_out_of_order` / `sweep_static`（3s+ 零几何变化会失败）；选择器写错会报 `motion_selector_missing`。
- **不要用** `snapshot` / `compare` / `grade-compare`：它们输出 PNG，本沙箱 `ReadFile` 只读文本，模型看不到。

# 报错修复

## 常见报错对照表

| 报错 | 修法 |
| --- | --- |
| `font_family_without_font_face` | 换成内置字体，或中文用 `@font-face` 的 `"CJK"` |
| `gsap_css_transform_conflict` | 删掉 CSS 里的 `transform` 初始值，把初始态写进 `fromTo` 的 from |
| `media_crossorigin_breaks_preview` | 删除 `crossorigin` 属性 |
| `media_missing_id` | 给 `<audio>` 加唯一 `id` |
| `timed_element_missing_clip_class` | host 槽位加 `class="clip"` |
| `root_composition_missing_duration_source` | 根 / 子根上写 `data-duration` |
| `gsap_repeat_ceil_overshoot` | `repeat` 用 `Math.max(0, Math.floor(...) - 1)` |
| `missing_gsap_script` | 根与子文件都用根相对 `vendor/gsap.browser.js`（子文件**不要** `../`） |
| `invalid_parent_traversal_in_asset_path` | 资源一律根相对；所有 composition 以项目根为 base URL |
| `duplicate_audio_track` | 重叠音频换一个 `data-track-index` 或去掉重叠 |
| `gsap_timeline_registered_before_async_build` | 异步构建完后再注册 `window.__timelines[...]` |
| `standalone_composition_wrapped_in_template` | 顶层 `index.html` 的根不要包 `<template>` |
| `subcomposition_root_styled_by_class` | 子文件根用 `#root` 设尺寸，不要用 class |
| 子文件样式丢失 / 空白 | 把 `<style>`/`<script>` 放进 `<template>` 内，不要放 `<head>` |
| host id ≠ 子 id | 槽位 `data-composition-id`、子根 id、`__timelines` 键三者同名 |
| 布局警告 `canvas_overflow`（装饰出血） | 给该装饰元素加 `data-layout-allow-overflow` |
| 布局警告 `content_overlap`（叠化期文字重叠） | 给参与叠化的文字块加 `data-layout-allow-overlap` |
| 对比度误报（幽灵字/纹理字） | 给装饰文字加 `data-layout-ignore` |
| Runtime 报 `gsap is not defined` | 检查对应文件的 `<script src="…gsap…">` |
| `video_nested_in_timed_element` | timed 的 `<video>` 不要再套在另一个 timed 元素里 |
| `audio_volume_tween_overrides_gain` | 音量动画与 `data-volume` 冲突，二选一 |
| `root_missing_width` / `root_missing_height` | 根元素补 `data-width` / `data-height` |

其他报错：读报错里的 `Fix:` 提示，按其修改；改完立即重跑 `check`。

## 失败降级（最多 3 轮）

1. 单镜子文件 `check` 报错 → 按对照表只修该文件，重跑。
2. 整项目 `check` 报错 → 先看是挂载 id 不一致还是某镜内容；定位后修对应文件。
3. 渲染失败（超时/编码）→ `--quality standard -w auto` 重试；仍失败 → `--fps 24 -w auto` 重试。若日志疑似 OOM / Chrome 崩溃 → 用 `--quality draft -w auto` 再试。
4. 仍失败 → 简化**出问题的那一镜**：去掉复杂 SVG / Canvas / 滤镜，只留文字 + 色块 + 基础位移/淡入，重渲。
5. 三轮后仍无 MP4：在聊天里报告错误与已尝试的修复，不要静默退出。

## 常见陷阱（速查）

1. 默认模块化：每镜 `compositions/shot-NN.html` + 薄 `index.html`；禁止单文件塞全部分镜。
2. 每个子文件写完就单独 check；整项目再 check 一次（跨文件挂载）。
3. 子文件：`<template>` 内放 style/script；根用 `#root`；三者 id 同名；局部时间从 0。
4. 旁白音频只在根；每个 `<audio>` 有 id；全局时间对齐时间轴表。
5. 根 `data-duration` 写死 TOTAL；不要用脚本改。
6. 半开窗口：动画收尾留 0.1–0.3s 余量。
7. 中文只走 `"CJK"`；数字用 `tabular-nums`。
8. 动画挂各自 `tl`；不要裸 tween、不要 `repeat: -1`、不要真实时钟。
9. 转场写在根时间线操作槽位；画面可重叠，音频不重叠。
10. 失败先改源码再重跑，不排查环境、不装包、不联网。
