# 站点定制 · 设置页信息架构

**原则：** 实例身份可定制，产品基因不可抹掉。  
Community 提供站名 / 文案 / 入口 / 轻度视觉；**不**提供完整 BrandLockup 白标替换、多租户 OEM。  
开源溯源：`source_url` + 可选 `oss_credit`；防滥用靠 AGPL，不靠强制恶心页脚。

| 状态 | 说明 |
|------|------|
| **L0** | **已交付** v0.9.2（`site_name` 进壳层字标等） |
| **L2 子集** | **已交付** v0.9.5（`theme_accent` / `theme_bg_*` / `theme_glass` + **外观** Tab） |
| **L1 / L2 余量** | 草案（文案包、默认色彩模式、PWA 动态、自定义 CSS） |

关联：v0.9.2 验收见 `docs/superpowers/plans/2026-08-04-v0.9.2-acceptance.md`；  
发版说明见 [CHANGELOG](../../CHANGELOG.md#095---2026-08-06)。

---

## 分层

| 层 | 目标 | 版本 |
|----|------|------|
| **L0** 诚实补齐 | 站名真正出现在壳层；设置说明作用面 | **0.9.2 · shipped** |
| **L1** 文案包 | 上传 slogan、登录说明、SEO、条款链接 | 0.9.x / 0.10（草案） |
| **L2** 轻视觉 | 强调色、站点背景图（遮罩）、毛玻璃、默认主题、PWA 名/色 | **0.9.5 · accent+bg+glass shipped**；其余 0.10+ |
| **L3** 运营增强 | 水印变量、错误页模板… | 按需（本稿不展开 UI） |

---

## 设置页 Tab 结构（当前实现）

与代码 `SettingsPage` 一致（`web/src/pages/admin/settings/`）：

```
系统设置
├── 基本 (basic)          ← 身份 + 注册/广场开关
├── 外观 (appearance)     ← L2：强调色 / 背景图 / 遮罩 / 面板不透明度  [v0.9.5]
├── 站点插槽 (slots)      ← 公告/页脚/关于/HTML/分享脚/favicon/source 等
├── 机审 / OCR / SMTP / 防盗链 / 处理  ← 运维能力
```

**尚未单独拆出**「公开文案 (copy)」Tab：L1 新键若落地，可从 slots 拆出或仍挂插槽。  
下文标「现有键 / 新键 / 已交付」。

---

## L0（0.9.2 · shipped）

| 设置项 | 键 | 位置 | 行为 |
|--------|-----|------|------|
| 站点名称 | `site_name` | 基本 | 标题、**顶栏字标**、页脚、邮件、OG、分享站名 |
| 站点名称说明 | —（i18n hint） | 基本 | 列清影响面；标明鱼标图形不替换 |
| Favicon | `favicon_url` | 插槽 | 仅图标；hint 与站名区分 |
| 页脚 credit | `oss_credit` | 插槽 | on\|off「基于 imgli」 |
| 对应源码 | `source_url` | 插槽 | AGPL 义务入口 |
| 分享页脚 | `share_branding` | 插槽 | off \| site \| links |

**壳层接站名：** Nav、GuestLayout、DiscoverLayout、Auth 壳、Admin 壳（站名 + ADMIN）、Share 顶栏。

**不做 L0：** 上传自定义 Logo、换 BrandMark path、manifest 全动态（可进 L2 余量）。

---

## L1（文案包 · 草案，未全量交付）

| 设置项 | 建议键 | Tab | 说明 |
|--------|--------|-----|------|
| 上传页副标题 | `upload_subtitle` locale map | 公开文案 | 覆盖默认 slogan |
| 登录页说明 | `login_notice` locale map | 公开文案 | 对称 `register_notice` |
| 注册说明 | `register_notice` | 插槽 / 公开文案 | **现有** |
| 帮助 URL | `help_url` | 插槽 / 公开文案 | **现有** |
| 升级/自托管 URL | `upgrade_url` | 插槽 / 公开文案 | **现有** |
| SEO 描述 | `meta_description` locale map | 公开文案 | SPA meta + 可选 OG |
| 默认 OG 图 | `og_image_url` | 公开文案 | http(s) |
| 服务条款 URL | `terms_url` | 公开文案 | 页脚/注册旁链 |
| 隐私政策 URL | `privacy_url` | 公开文案 | 同上 |
| 关于页 | `about_enabled` / `about_body` | 插槽 | **现有** |
| 公告 | `announcement` | 插槽 | **现有** |
| 页脚链接组 | `footer` | 插槽 | **现有** |
| HTML 注入 | `html_inject` | 插槽 | **现有**；高危提示保留 |

L1 验收口径：改文案后上传页/登录页/注册页/SEO 可见，不破坏产品默认。

---

## L2（轻视觉）

### 已交付（v0.9.5）

| 设置项 | 键 | Tab | 行为 |
|--------|-----|------|------|
| 强调色 | `theme_accent` | 外观 | 空或 `#RGB` / `#RRGGBB` → 主按钮 `--btn` / `--btnText` |
| 站点背景图 | `theme_bg_image_url` | 外观 | 空或 http(s) / 站内路径；整站 cover |
| 背景遮罩 | `theme_bg_dim` | 外观 | 0–1，默认 **0.72** |
| 面板不透明度 | `theme_glass` | 外观 | 0–1，默认 **0.78**；有背景图时毛玻璃（`backdrop-filter`），边框随 glass 变淡 |

**实现要点（与代码一致）：**

- Settings 常量：`internal/model` `SettingTheme*`；校验 `adminsvc/theme.go`。
- 管理端读写：`adminsvc.GetSettings` / `PutSettings`；前端 `AppearanceTab`。
- 公开下发：`GET /api/v1/config` 含上述四键。
- 应用：`web/src/lib/siteTheme.ts` → `applySiteTheme` 写 `body` CSS 变量与 `data-bg-image`。
- 暗色语义色：`tokens.css` 使用 `@theme inline`，由 `body[data-theme=dark]` 运行时翻转。

### 草案（未交付）

| 设置项 | 建议键 | Tab | 说明 |
|--------|--------|-----|------|
| 默认色彩模式 | `theme_default` | 外观 | light \| dark \| system（用户偏好仍本地优先） |
| PWA 短名 | 派生自 `site_name` 或 `pwa_short_name` | 外观 | manifest `short_name` |
| PWA theme_color | `pwa_theme_color` | 外观 | 可与 accent 同源 |
| 自定义 CSS URL | `custom_css_url` | 外观 | 可选；管理员自担；CSP 注意 |

**硬约束：** BrandMark（鲤鱼）路径/SVG **不可配置**。字标可用 `site_name`；图形保持产品识别。

---

## 字段归属一览

| 键 | 今日 Tab | 备注 |
|----|----------|------|
| `site_name`, registration_mode, guest_upload, plaza | 基本 | L0 |
| favicon, source, oss_credit, share_branding, about*, footer, announcement, html_inject, help, upgrade, register_notice | 插槽 | 部分可进 L1 拆分 |
| `theme_accent`, `theme_bg_image_url`, `theme_bg_dim`, `theme_glass` | **外观** | **L2 · 0.9.5** |
| moderation, ocr, smtp, hotlink, processing | 各自 Tab | 运维 |
| L1 新键 / L2 余量 | — | 公开文案 / 外观（未来） |

---

## 权限与安全

- 全部仅 **admin** 可写。  
- `html_inject` / 未来 `custom_css_url`：持久警告「错误或恶意代码影响全站」。  
- URL 字段：空或 http(s) / 站内路径；`theme_accent` 仅 hex。  
- 不在 Community 提供「隐藏所有 imgli 标识且无法 `source_url`」的一键模式。

---

## 与商业白标边界

| Community | Commercial / 非目标 |
|-----------|---------------------|
| 站名字标、favicon、文案、轻色/背景 | 替换 BrandMark、多品牌锁标包 |
| oss_credit 可关 | 合同级去开源标识 |
| 单实例 settings | 多租户控制面 |

详见 `COMMERCIAL.md`、`ROADMAP.md` Non-goals。
