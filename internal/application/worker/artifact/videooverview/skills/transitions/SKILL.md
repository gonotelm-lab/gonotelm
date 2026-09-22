---
name: transitions
description: 场景转场：cut/crossfade/blur crossfade/whip pan/zoom-through/推挤，速度匹配与全片转场纪律。设计镜间衔接或想让转场更有电影感时读。
---

# 场景转场

本技能是 `video-generate` 入口提示的知识库（位于工作目录 `skills/transitions/SKILL.md`）。
基础重叠规则（crossfade 0.4s / wipe·slide 0.5s）见入口「时间轴重建」；本技能讲怎么让转场更有表现力。

## 纪律

- 转场 = 上一镜退场与下一镜入场**同时发生**；写在**根**时间线上操作 host 槽位或槽位内层包装，不要进子文件改兄弟镜。
- 音频窗口**不重叠**；画面可重叠。
- 全片**一个主转场**（60–70% 的镜界）+ 最多一个强调转场；同题递进用 `cut`，大落差才用明显转场。
- 退场用 `.in`（加速离开），入场用 `.out`（减速到达）；在 cut 处两侧速度要匹配（±5%）。
- 时长预设：snappy 0.2s / smooth 0.4s / gentle 0.6s / dramatic 0.5s / instant 0.15s / luxe 0.7s。

## 速度匹配转场（最像「一个镜头」）

退场加速 + 模糊，入场从模糊中减速；最快点交汇在 cut 上。

```js
const T = aWindowEnd - 0.33;
tl.to("#el-shot-01", { y: -150, filter: "blur(30px)", opacity: 0, duration: 0.33, ease: "power2.in" }, T);
tl.fromTo("#el-shot-02", { y: 150, filter: "blur(30px)", opacity: 0 }, { y: 0, filter: "blur(0px)", opacity: 1, duration: 1.0, ease: "power2.out" }, T);
```

- **blur crossfade**（最常用）：medium 能量 8–15px / 0.4–0.6s / hold 0.1–0.2s；calm 20–30px / 0.8–1.2s；high 3–6px / 0.2–0.3s。对槽位内层做 `filter: blur()` + opacity。
- **whip pan**：退场 `x:-400, blur(24px), 0.3s power3.in` → 入场 `x:400→0, blur→0, 0.3s power3.out`。
- **zoom-through**：退场 `scale:1→1.2, blur(20px), 0.2s power3.in` → 入场 `scale:0.75→1, blur→0, 0.5s expo.out`。

## 按叙事位置选

| 位置 | 转场 |
| --- | --- |
| 开场 | 特色转场 0.4–0.6s（blur crossfade / zoom-through） |
| 相关点之间 | 主转场 0.3s（cut 或 crossfade） |
| 换主题段 | 换一个明显转场 + 足够 `pause_after` |
| 高潮 | 最快最猛（whip / zoom-through） |
| 收尾 | 温和 0.5–0.7s |
| 片尾 | 最慢 0.6–1.0s（淡出到定格） |

## 已知坑

- 叠加层要**全屏 1920×1080**，不要细条；glitch 类 RGB 叠加用 normal 混合 35%（`multiply` 在暗底不可见）。
- z-order：gravity drop / zoom-out / 对角分割需要**退场在上**（`zIndex: 10`），入场在下（`zIndex: 1`）。
- 需要隐藏上一镜时用 `tl.set` 在转场结束点（**不要** `onComplete`），保证回退 seek 正确。
- 本沙箱没有 shader 转场包；不要用 star iris（多边形插值坏）、tilt-shift（没有选择性模糊）、lens flare、hinge / door。
- 叠加层可能触发 `content_overlap`，给参与叠化的文字加 `data-layout-allow-overlap`。

## 自检

- [ ] 退场与入场同 T 发生，无「先淡出再入场」；
- [ ] 全片主转场一致，强调转场 ≤1 个；
- [ ] 音频窗口不重叠，转场只操作槽位 / 内层包装；
- [ ] 模糊在镜内释放干净，下一镜内容清晰；
- [ ] 叠化文字已加 `data-layout-allow-overlap`。
