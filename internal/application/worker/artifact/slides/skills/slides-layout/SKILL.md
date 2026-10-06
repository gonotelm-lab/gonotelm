---
name: slides-layout
description: 版式预算纪律「先算后画」：画布/边距/页脚带的比例推导、游标切带、嵌套 innerH、可行性门闩、循环不变量与六类反模式。写任何 createSlide 的 x/y/w/h 之前必读。
---

# 版式预算：先算后画（强制）

本技能是 slides 生成入口提示的知识库（位于工作目录 `skills/slides-layout/SKILL.md`）。
**违反本文件的写法即视为失败**，与页面好不好看无关：坐标错误一定会在成品里表现为重叠或溢出。

画布固定 `LAYOUT_16x9` = **PAGE_W 10 × PAGE_H 5.625 英寸**。单位是英寸，数值直接写。

边距、圆角、间距的比例**不自己定**：全部取 `skills/theme/<visual_style>/theme.json` 的 `space` / `radius`（写页前先读该文件）。把它们乘上 `PAGE_W` / `PAGE_H` / 卡片短边得到本页常量，再用于绘制；全文保持一致。禁止抄写散落的绝对坐标。

```javascript
// 比例来自 tokens.space / tokens.radius（单位说明见 theme.json 的 units）
const marginX  = PAGE_W * tokens.space.marginX;
const marginY  = PAGE_H * tokens.space.marginY;
const footerH  = PAGE_H * tokens.space.footerH;
const gapX     = PAGE_W * tokens.space.gapX;
const gapY     = PAGE_H * tokens.space.gapY;
const pad      = 卡片短边 * tokens.space.pad;
const radius   = 卡片短边 * tokens.radius.card;
```

## 每个 createSlide 必须分三段：算完再画

禁止凭感觉写死坐标，禁止先 `addShape`/`addText` 再补算。

```text
① 画布与边距
   PAGE_W, PAGE_H          ← LAYOUT_16x9：10 / 5.625
   marginX, marginY        ← PAGE * tokens.space.marginX / marginY
   footerH                 ← PAGE_H * tokens.space.footerH
   contentLeft  = marginX
   contentRight = PAGE_W - marginX
   contentWidth = contentRight - contentLeft
   contentBottom = PAGE_H - footerH
   // 任意内容元素：x≥contentLeft, x+w≤contentRight, y+h≤contentBottom
   // 页脚带仅页码（及可选极短来源）；任意元素 y+h ≤ PAGE_H

② 本页结构预算（优先用游标 / 剩余高度，不要用互不相关的百分比硬叠）
   titleY, titleH
   let cursorY = titleY + titleH + gapAfterTitle
   contentBottom 固定后：每一水平带只吃「当前剩余」
     remaining = contentBottom - cursorY
     画完一带：cursorY += bandH + gapY
   // 单带内纵向 N 项：
   step = remaining / N; cardH = step - gapY
   // 横向 K 列：
   colW = (contentWidth - (K-1)*gapX) / K
   // 页码：仅用页脚带推导，不占用 cursorY

   **嵌套规则（高频事故，必须遵守）**
   父容器确定后先定 inner：
     innerTop    = parentY + pad
     innerBottom = parentY + parentH - pad
     innerH      = innerBottom - innerTop
   子列表 / 子卡只能在 [innerTop, innerBottom] 内分配：
     childStep = innerH / n   （或先扣掉标题带高度再除）
   禁止：子项起点已在 parent 中部，却仍用「整段 parentH - pad」当 usable → 必溢出。
   禁止：外卡 height = 整段 usableH，同时又在卡内下部叠一条「底栏 strip」且未先从 usableH 扣掉 stripH。
   多水平带（上卡+中条+下区）：先按剩余依次切 bandH，最后一带 = contentBottom - cursorY。

③ 可行性门闩（不通过则禁止开画）
   任一元素：x≥contentLeft, x+w≤contentRight, y≥0, y+h≤contentBottom（页码可在页脚带）
   子元素完全在父 inner 内；内容框互不重叠（底衬叠字除外）
   放不下 → 减 N/K、删字、拆页；禁止压 gap、禁止画进页脚带
```

## 带划分

| 带 | 含义 | 谁能放 |
|----|------|--------|
| 内容区 | 标题下至 `contentBottom`，`x` 在左右边距内 | 正文 / 卡片 / 图表 |
| 页脚带 | `contentBottom`～`PAGE_H` | **仅**页码；可选一行极短来源 |
| 页码 | 由 `PAGE_W` / `PAGE_H` / `footerH` / `contentRight` 推导 | 页码专用 |

页码（封面除外）放在页脚带右下，只显示当前页（`3` 或 `03`，不要 `3/12`）；圆点或胶囊均可，位置与尺寸必须由本页布局常量推导。

## 循环不变量

`forEach` / `for` **每一轮用公式推进**（`y_i` / `x_i` / `cursorY`），禁止循环内写死同一坐标。

```javascript
// 纵向：用游标或 step
let y = y0;
items.forEach((it) => {
  // draw at y with height cardH
  y += step;
});
// 横向
items.forEach((it, i) => {
  const x = contentLeft + i * (colW + gapX);
});
```

小节标题与其下首块：首块起点 ≥ 标题底边 + 间隙。

## 反模式（实装中已出现，看到就改）

1. **先画后算**：未写布局常量就 `addText` / `addShape`。
2. **魔法坐标**：抄绝对 `x/y/w/h`，或循环内坐标不递增。
3. **嵌套双重计价**：子栈起点在父中部，usable 却按整段父高计算 → 末项越出 `contentBottom`。
4. **满高卡 + 底 strip**：`rowH = usableH` 后再画 `stripY = y0 + 0.72*rowH`，与卡内列表重叠 / 顶出。
5. **百分比区域互撞**：多个 `* 0.14 / 0.18 / 0.72` 区域未保证之和 ≤ 1，也未扣 pad。
6. **标题吃首卡 / 双底栏**：标题与首块同 y；或总结条与页码抢页脚带。

## 记住一条

宁可**减项、删字、拆页**，也不要压 `gap`、缩字号硬塞、或让元素越界。版式算不下的信号永远是"内容太多"，不是"还差 0.02 英寸"。
