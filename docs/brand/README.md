# img.li · Logo Handoff

品牌名「图鲤」（读 tú-lǐ），域名即字标 `img.li`（img = 图，li = 鲤）。视觉：一条几何白鲤上跃，图片一跃成外链。全套**单色**，与图床产品设计系统同源。

## 概念

- **鱼身**：菱形 = 图片文件的矩形，旋转 45° 表示"跃动"。
- **上尾鳍**：左下折角，鲤鱼摆尾上跃。
- **顶部方点**：溅起的水花 / 上传落点，同时就是字标 `img·li` 里的那个分隔点——图形与网址合一。
- **鱼眼**：鱼身内一处方形缝隙（导出为**透明镂空**，`fill-rule="evenodd"`），因此在任意底色上都能正确显示。

## 文件（`svg/`，矢量，可无损缩放）

| 文件 | 用途 |
|---|---|
| `mark-dark.svg` | 主标·实心，深色（浅底用）；鱼眼透明镂空 |
| `mark-light.svg` | 主标·实心，白色（深底用） |
| `mark-line-dark.svg` / `mark-line-light.svg` | 描边版（仅 ≥48px 使用） |
| `lockup-dark.svg` / `lockup-light.svg` | 横向锁标：鱼 + 等宽字标 `img.li`（中点 muted） |
| `icon-ondark.svg` / `icon-onlight.svg` | 应用图标（128 圆角瓦片，圆角 28/128 ≈ iOS 比例） |
| `favicon-ondark.svg` / `favicon-onlight.svg` | 16–32px 小尺寸：鱼眼改为实色填充（镂空在极小尺寸易糊） |

`img.li Logo 展示.dc.html` 是可交互展示页（浅/深主题切换、权重、应用图标各档、导航实景、规范），浏览器直接打开（需同目录 `support.js`）。

## 颜色（跟随产品 token，切勿上彩）

| 场景 | 图形/字标 | 底色 | 中点 |
|---|---|---|---|
| 浅色 | `#17171a` | `#ffffff` / `#fafafa` | `#77777f` |
| 深色 | `#ffffff` | `#17171a` / `#0f0f11` | `#8e8e97` |

## 字标

- 等宽体：`ui-monospace, 'SF Mono', Menlo, Consolas, monospace`，800，letter-spacing ≈ -0.03em。
- 写法固定小写 `img.li`；中间的点用 muted 色弱化，让 `img` / `li` 两段清晰、便于读作「图鲤」。
- 无法用矢量时，降级为单独的鲤鱼图形块（如 22px 方块）。

## 使用规范

- **单色 only**：只用 `--text` 或反白，禁止渐变/彩色/低对比灰。
- **最小尺寸**：主标 20px；favicon 16px 用实色眼版本（`favicon-*`）。
- **权重选择**：≤32px 用实心版；描边版仅 ≥48px。
- **留白**：四周留白 ≥ 鱼身高度的 40%，勿贴边或与文字挤压。
- **底色对比**：深底用 light 版，浅底用 dark 版。
- **导航栏组合**：鱼图形 20px + 等宽 `img.li` 站名 + 可选 `BETA` 描边徽章（见展示页第 04 节）。

## 生成位图 / favicon.ico（可选）

SVG 已是生产可用格式。如需 PNG/ICO：
```
# 需本地安装 rsvg-convert 或 inkscape
rsvg-convert -w 512 -h 512 svg/icon-ondark.svg -o icon-512.png
rsvg-convert -w 32  -h 40  svg/favicon-onlight.svg -o favicon-32.png
```
Web 端可直接 `<link rel="icon" href="favicon-onlight.svg">`（现代浏览器支持 SVG favicon），并配 `media="(prefers-color-scheme: dark)"` 切换深浅版。

## viewBox 速查（自行改版时）

`0 0 80 100`：水花点 `rect x62 y8 w12 h12 rx2` · 鱼身 `M44 30 L74 60 L44 90 L14 60 Z` · 鱼眼镂空 `M38 52 h8 v8 h-8 Z`（evenodd）· 尾鳍 `M14 60 L2 48 L6 60 L2 72 Z`。
