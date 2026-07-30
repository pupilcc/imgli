# 存储能力矩阵（Caps）、策略 UI 与 FTP 双轨

**状态**: draft v2（已吸收 codex 审阅 + 产品拍板）  
**日期**: 2026-07-30  
**范围**: Caps 契约、策略 DTO/UI/doctor、compat 驱动（含 FTP）、公开文档  
**相关**: [实现清单](storage-caps-impl-checklist.md) · [s3-compatibility.md](../s3-compatibility.md) · [codex 审阅](storage-caps-draft.codex-review.md)

---

## 0. 产品拍板（双轨）

### 0.1 推荐路径 vs 二进制内置

| 路径 | 定位 | 谁维护复杂度 |
|------|------|----------------|
| **外置优先**：OpenList / rclone / 面板代理 FTP → WebDAV 或本地/S3 → imgli | **文档与架构默认推荐** | 用户中间件；imgli 零 FTP 协议债 |
| **内置 compat**：`internal/storage/ftp` + Caps `tier=compat` + **零 serve 特判** | **获客 + 减少用户多跑一个组件** | imgli 承担协议边角；功能显式受限 |

两者不矛盾：

- **「优先」** = 生产推荐与叙事默认，不是「我们不做 FTP」。  
- **「落地」** = 可选兼容能力，不是第二套存储中枢，也不是 first-class。

### 0.2 解耦硬约束（FTP 与一切 compat 驱动）

1. **独立包**：如 `internal/storage/ftp`，仅实现 `storage.Driver`（+ 可选 `CapabilityProvider`）。  
2. **Caps 标注**：`tier=compat`，无预签名、CDN 不推荐、热读不推荐、安全提示。  
3. **零 serve 特判**：`handler/serve.go` **不得**出现 `driver == "ftp"`；继续只认 `Presigner` 类型断言与 `cdn_domain`/`streamFile` 通用逻辑。  
4. **可删除性**：去掉注册 + 包 + UI 分支 + i18n 后，`go test ./...` 仍绿。  
5. **范围咬死**：Put/Open/Delete/Exists + 连接探测；不做 List 热路径、多分片产品、与 S3 对等能力。

### 0.3 Codex 开放问题拍板（已定）

| # | 问题 | 决定 |
|---|------|------|
| 1 | local/webdav `public_cdn_offload` | **false + warning**；UI 文案「不推荐/不保证」，非「协议上不能 302」 |
| 2 | warnings 来源 | **P0 后端返回** `caps`/`tier`/`warnings`；前端可镜像未保存表单 |
| 3 | Catalog API | **非 P0**；UI 硬编码驱动列表直至 Registry 落地 |
| 4 | compat 启用 ack | **要**；审计 `policy_enable_compat` |
| 5 | FTP 未实现时 UI | **不展示「即将推出」**；帮助文案写「暂不支持；优先外置或迁移」直至驱动合并 |

---

## 1. 背景与问题

### 1.1 仓库现状（锚点）

| 位置 | 现状 |
|------|------|
| `internal/storage.Driver` | 必选：`Put` / `Open` / `Delete` / `Exists` |
| `storage.Presigner` | 可选：类型断言；仅 S3；`PresignTTL=60s` |
| `storagesvc.Resolver.Driver` | `switch`：`local` \| `s3` \| `webdav` |
| `StoragePolicy.CDNDomain` | 策略字段；公开原图 `/i` 的 302 目标（`ObjectURL`） |
| S3 `presign_domain` | **config map**；未配则私密图走流式 |
| `handler/serve.go` | 公开+CDN → 302；私密+Presigner → 短签 302；否则 `streamFile`；**缩略图始终 app 流式** |
| `PoliciesPage` | 驱动：本地 / S3 / WebDAV；无能力说明 |
| `validateDriverConfig` | 仅 config 形状 |
| 公开文档 | 见 `s3-compatibility.md`（v2 起改为双轨表述） |

产品问题：

1. 仅 FTP 供应商（如部分虚拟主机 / Namecrane）需要接住，但能力不能与 S3 对等宣传。  
2. local/WebDAV **静默**无预签名；CDN 任意驱动可填、无告知。  
3. 外置代理（OpenList）已能覆盖部分用户；内置 FTP 服务「单二进制」获客，必须可拆卸。

### 1.2 设计原则

1. **能力是数据**：同一套 Caps 供 admin API、UI、doctor 消费。  
2. **Caps 在 P0 为 advisory 元数据**，不替代 serve 控制面（避免双源真相）。  
3. **丢失功能一等公民**：配置可见、compat 启用确认、列表徽章。  
4. **Tier 分层**：`first_class` / `supported` / `compat` / `migrate_only`。  
5. **最小契约**：`Driver` 四方法；无 List/Multipart 产品接口则 Caps 标 false。  
6. **单二进制进程内驱动**；不做 Go `.so` plugin。  
7. **静态能力 vs 生效态分离**（§2.3）。

---

## 2. Caps 与生效态

### 2.1 包位置

`internal/storage`（与 `Driver`/`Presigner` 同包）。

### 2.2 类型定义

```go
package storage

type Tier string

const (
	TierFirstClass  Tier = "first_class"   // local、s3
	TierSupported   Tier = "supported"    // webdav
	TierCompat      Tier = "compat"       // ftp 等
	TierMigrateOnly Tier = "migrate_only" // 预留：仅导入
)

// Caps 驱动级静态能力（与策略配置无关的「默认画像」）。
// ObjectCRUD 由 Driver 接口隐含，不重复字段。
type Caps struct {
	Tier       Tier   `json:"tier"`
	SummaryKey string `json:"summary_key"` // i18n key

	// 驱动是否「通常」以 TLS 提供远程传输。local=true（无网络）。
	// 实际某策略是否 https 见 Effective.TransportIsTLS。
	TransportTLSPreferred bool `json:"transport_tls_preferred"`

	AllowsInsecure bool `json:"allows_insecure"` // 是否允许 allow_insecure 类配置

	// 下列仅当存在真实可选接口/实现时才可为 true。
	// P0：全驱动 RangeGet 按实测；ListPrefix/MultipartUpload 一律 false
	//（当前 Driver 契约无 List/Multipart 方法）。
	RangeGet         bool `json:"range_get"`
	ListPrefix       bool `json:"list_prefix"`
	MultipartUpload  bool `json:"multipart_upload"`

	// 是否「推荐/保证」作为对象 CDN 匿名回源。false 不表示 serve 禁止 302：
	// 今日 cdn_domain 非空时任意驱动都可能 302（driver-agnostic）。
	PublicCDNOffloadRecommended bool `json:"public_cdn_offload_recommended"`

	// 驱动是否实现 Presigner 接口（与类型断言一致）。
	// 运行时是否真能签出 URL 还取决于策略 config（如 presign_domain）——见 Effective。
	PrivatePresignCapable bool `json:"private_presign_capable"`

	HotPathOK bool `json:"hot_path_ok"`

	FeatureLossKeys []string `json:"feature_loss_keys"`
}

// Effective 策略级生效态（Caps + 该策略 config/cdn_domain 叠加）。
type Effective struct {
	Caps Caps `json:"caps"`

	// 远程 endpoint 是否为 https（local 恒 true）。
	TransportIsTLS bool `json:"transport_is_tls"`

	// 公开原图是否会因 cdn_domain 非空而尝试 302（与 driver 无关）。
	PublicCDNRedirectConfigured bool `json:"public_cdn_redirect_configured"`

	// PrivatePresignCapable && 策略具备签名所需配置（S3: presign_domain 等）。
	PrivatePresignReady bool `json:"private_presign_ready"`

	// 供 UI/doctor 的结构化警告（后端权威）。
	// Warnings 也可挂在 DTO 顶层；此处便于单测。
}

// CapabilityProvider 可选：实例声明 Caps（否则查静态表 capsByDriver）。
type CapabilityProvider interface {
	Capabilities() Caps
}

// CatalogEntry 预留 Registry；P0 不暴露 Catalog HTTP API。
type CatalogEntry struct {
	Driver   string `json:"driver"`
	LabelKey string `json:"label_key"`
	Tier     Tier   `json:"tier"`
	Caps     Caps   `json:"caps"`
}
```

**命名**：原草案 `Capabiliator` 废止，使用 `CapabilityProvider`。

### 2.3 静态 vs 生效（防混读）

| 问题 | 读哪里 |
|------|--------|
| 这驱动支不支持预签名接口？ | `Caps.PrivatePresignCapable` |
| 这策略现在会不会短签 302？ | `Effective.PrivatePresignReady` |
| 适不适合当 CDN 源？ | `Caps.PublicCDNOffloadRecommended` |
| 会不会 302？ | `Effective.PublicCDNRedirectConfigured`（仅表示配了域名） |
| 传输是否加密？ | `Effective.TransportIsTLS` |

### 2.4 与 `Presigner` / serve

| 机制 | 保留 | 说明 |
|------|------|------|
| `Presigner` 断言 | 是 | serve **唯一**决定是否短签的代码路径 |
| `Caps.PrivatePresignCapable` | 是 | UI/校验/doctor；单测与实现集合一致 |
| CDN | 策略字段 | **非**驱动方法；Caps 只表达推荐度 |
| 缩略图 `/t` | 始终流式 | Caps/CDN/预签名文案 **仅针对原图 `/i`** |

### 2.5 API 形状（P0）

`GET|POST|PATCH /api/v1/admin/policies`（及 list 每项）扩展：

```json
{
  "id": 1,
  "name": "…",
  "driver": "webdav",
  "config": "{…}",
  "cdn_domain": "",
  "path_template": "{Y}/{m}/{d}/{uniqid}.{ext}",
  "enabled": true,
  "file_count": 0,
  "used_bytes": 0,
  "created_at": "…",
  "tier": "supported",
  "caps": { },
  "effective": {
    "transport_is_tls": true,
    "public_cdn_redirect_configured": false,
    "private_presign_ready": false
  },
  "warnings": [
    {
      "code": "cdn_not_recommended",
      "message_key": "adminB.warnCdnWithoutCap",
      "severity": "warning"
    }
  ]
}
```

`warnings` 由后端在 list/create/update 时计算（权威源）。  
**不做** P0：`GET /api/v1/admin/storage-drivers`（Catalog）。

### 2.6 `cdn_domain` 校验（写入规范，P0 或紧随）

今日几乎原样拼接。规范：

| 规则 | 要求 |
|------|------|
| 空 | 允许（不 302） |
| scheme | 仅 `http` / `https` |
| host | 必填 |
| userinfo | 禁止 |
| query / fragment | 禁止 |
| path | **允许**作为对象键前缀前的 URL 前缀（与现 `ObjectURL` 拼接语义一致）；实现时单测锁定 |

无效 → `400` + `ErrBadConfig`（或等价）。

### 2.7 配置冲突 → warnings（P0 返回，默认不 400）

| 条件 | code | severity |
|------|------|----------|
| `cdn_domain != ""` && `!PublicCDNOffloadRecommended` | `cdn_not_recommended` | warning |
| S3 且 `PrivatePresignCapable` 且无 `presign_domain` | `presign_unconfigured` | info |
| `!TransportIsTLS` && 远程驱动 | `insecure_transport` | warning |
| `tier=compat` && 为唯一 enabled 策略 | `compat_only_policy` | warning（list/doctor 为主） |

硬门禁（无 cap 禁止 CDN 等）→ 后期可选 Phase，不在 P0。

### 2.8 解析辅助

```go
func CapsForDriver(driver string) (Caps, error)           // 静态表
func (r *Resolver) CapsFor(p *model.StoragePolicy) (Caps, error)
func (r *Resolver) EffectiveFor(p *model.StoragePolicy) (Effective, error)
```

P0 可用静态 `capsByDriver`；P2 再 `Register`。

---

## 3. 静态能力矩阵（P0 与代码对齐）

| 字段 | local | s3 | webdav | ftp（compat，实现期） |
|------|:-----:|:--:|:------:|:---------------------:|
| `tier` | first_class | first_class | supported | compat |
| `transport_tls_preferred` | true | true | true | true（要求 FTPS 为默认文档；明文仅 allow_insecure） |
| `allows_insecure` | false | false | false | true |
| `range_get` | true | true | true | false（弱/全量读） |
| `list_prefix` | **false** | **false** | **false** | **false** |
| `multipart_upload` | **false** | **false** | **false** | **false** |
| `public_cdn_offload_recommended` | false | true | false | false |
| `private_presign_capable` | false | true | false | false |
| `hot_path_ok` | true | true | true | false |

> List/Multipart：SDK 或未来可能有，**在 Driver 契约未暴露前一律 false**（codex IMPORTANT#1）。

### 3.1 `feature_loss_keys`

**local**: `storage.loss.no_presign`, `storage.loss.cdn_not_typical`  
**s3**: `[]`  
**webdav**: `storage.loss.no_presign`, `storage.loss.cdn_not_typical`, `storage.loss.vendor_semantics`  
**ftp**: 上列 + `storage.loss.no_cdn_offload`, `storage.loss.hot_path`, `storage.loss.ftp_security`, `storage.loss.ftp_reliability`

### 3.2 Summary keys

| driver | summary_key | 中文意图 |
|--------|-------------|----------|
| local | `storage.caps.summary.local` | 本地盘，流量默认走应用 |
| s3 | `storage.caps.summary.s3` | S3：可 CDN 卸带宽 + 私密预签名 |
| webdav | `storage.caps.summary.webdav` | WebDAV / 网盘 / **OpenList 等代理出口** |
| ftp | `storage.caps.summary.ftp` | FTP 兼容层；仅当无法外置时 |

---

## 4. 策略 UI

### 4.1 结构

保持左列表 + 右表单。增量：

- 列表：`tier` 徽章（兼容层 warning）  
- 表单：`PolicyCapsPanel`（等级、摘要、布尔能力、limitations）  
- CDN 字段下方：按 `caps`/`warnings` 显示劝退文案（**仅原图 `/i`**）  
- `presign_domain`：仅 `private_presign_capable`（S3 表单）  
- compat 启用：ack 确认 + 审计  

### 4.2 驱动分组（有 FTP 后）

- **推荐**：本地、S3  
- **正式支持**：WebDAV（文案可提 OpenList）  
- **兼容与传统**：FTP（compat）  

FTP **未合并前**：不出现在选择器；帮助区一句「FTP：请用 OpenList/外置代理，或等待/使用 compat 驱动文档」。

### 4.3 关键 i18n 意图（实现时写入 `adminB` + `storage`）

- tier / caps 面板 / warnings（含 `warnCdnWithoutCap`）  
- `confirmCompatEnable*`  
- `storage.caps.summary.*` / `storage.loss.*`  
- 帮助：`storage.help.ftpPreferProxy` — **优先 OpenList/rclone 等将 FTP 转为 WebDAV 或本地，再使用 imgli；内置 FTP 为功能受限兼容层。**

CDN warning 必须写清：配置后**可能**对公开原图 302，但不保证匿名可读；多数场景带宽仍可能打在应用；**不适用于缩略图**。

### 4.4 空状态

生产推荐本地或 S3；WebDAV 适合网盘/OpenList；FTP 为兼容层（见帮助链接）。

---

## 5. `imgli doctor`（P0 收窄）

| check id | 条件 | 级别 |
|----------|------|------|
| `storage_caps` | 每个 enabled 策略 | OK：driver/tier/presign_capable/cdn_recommended |
| `cdn_vs_caps` | cdn 已配 && !recommended | WARN |
| `compat_only` | 全部 enabled 为 compat | WARN |
| `insecure_transport` | 远程 && !TLS | WARN |
| `presign_unconfigured` | s3 无 presign_domain | INFO |

与现有 `cdn_metering` 合并文案，避免重复恐吓。  
P0 **不做**「presign 配置了但探测失败」的深探针（可留给 TestPolicy / 后续）。

---

## 6. FTP 双轨（文档 + 实现）

### 6.1 用户决策树

```text
供应商只给 FTP？
├─ 能跑 OpenList / rclone / 面板 WebDAV？
│    └─ 是 → 【推荐】代理后用 imgli WebDAV 或同步到 local/S3
└─ 必须单二进制、拒绝中间件？
     └─ 是 → imgli 内置 FTP compat（功能受限，见 Caps）
```

### 6.2 内置 FTP 范围

| 做 | 不做 |
|----|------|
| FTPS 优先；明文仅 `allow_insecure=true` + warning | 当 first_class 宣传 |
| Driver 四方法 + TestPolicy 探针 | serve/CDN/缩略图 FTP 分支 |
| Caps + UI 徽章 + 启用 ack | List/同步/多副本/Telegram 式能力 |
| 被动模式等最小可用配置项 | 与 S3 预签名/CDN 对等 |

### 6.3 与公开文档

- `docs/s3-compatibility.md`：Out of scope 改为 **FTP 非 first-class；推荐外置；compat 见清单**。  
- 可选后续：`docs/storage-ftp.md` 操作说明（实现合并时写）。

---

## 7. 分阶段

| Phase | 交付 | 说明 |
|-------|------|------|
| **P0** | 静态 Caps + Effective + DTO warnings + UI 卡片 + doctor 收窄 + 文档双轨 | advisory；三驱动 |
| **P1** | `cdn_domain` 严格校验；compat ack 审计钩子就位 | 无 FTP 也可先做 ack 框架 |
| **P2** | Registry 包裹 switch（可选 Catalog API） | 为 ftp 注册铺路 |
| **P3** | `internal/storage/ftp` + 表单 + 单测/集成 + Namecrane 类实测 | 零 serve 特判 |
| **P4** | 可选硬门禁 | 需迁移说明 |
| **P5** | 进程外 sidecar | 远期，非本范围 |

详细勾选见 [storage-caps-impl-checklist.md](storage-caps-impl-checklist.md)。

---

## 8. 明确不做

- Go `plugin` 热加载、远程下载第三方驱动  
- Telegram 等 first_class  
- 可配置 `PresignTTL`  
- 用 Caps 布尔替代 serve 的 `Presigner` 断言  
- P0 禁止 local/webdav 的 `cdn_domain`（仅 warning）  
- UI「即将推出 FTP」徽章（未实现前）

---

## 9. 验收

### P0

1. `GET /api/v1/admin/policies` 每项含 `tier`、`caps`、`effective`、`warnings`。  
2. 切换 local/s3/webdav，能力卡片与 §3 矩阵一致。  
3. local/webdav 填 CDN → 后端 warning + UI 展示。  
4. 单测：`PrivatePresignCapable` ⇔ 实现 `Presigner`。  
5. 中英文 i18n 无 TSX 硬编码中文。  
6. 文档：OpenList/外置优先 + 内置 FTP 为未来/compat 叙事一致。

### P3（FTP）

1. 无 `serve.go` 中 ftp 字符串分支（rg 验收）。  
2. 启用需 ack；审计 `policy_enable_compat`。  
3. Caps 全 loss keys 可见；`hot_path_ok=false`。  
4. TestPolicy 对 FTPS 探针成功路径有单测或集成说明。  
5. 删除 ftp 包与注册后主测试仍通过（可删除性抽检）。

---

## 10. 参考路径

- `internal/storage/storage.go`  
- `internal/service/storagesvc/storagesvc.go`  
- `internal/handler/serve.go`  
- `internal/handler/admin_policies.go`  
- `internal/service/adminsvc/policies.go`  
- `web/src/pages/admin/policies/PoliciesPage.tsx`  
- `web/src/i18n/locales/{zh,en}/adminB.ts`  
- `docs/s3-compatibility.md`  
- `docs/design/storage-caps-impl-checklist.md`  
