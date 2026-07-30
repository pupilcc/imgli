# 实现清单：Storage Caps + FTP 双轨

**依据**: [storage-caps-draft.md](storage-caps-draft.md) v2  
**目标**: Caps/UI 告知 → 可选 FTP compat；文档推荐 OpenList/外置优先  
**约束**: 独立驱动包 · Caps 标注 · **零 serve 特判** · 可删除

图例：`[ ]` 未做 · `[x]` 完成 · `[-]` 取消

---

## 文档与叙事（可先于代码合并）

- [x] 更新 `docs/s3-compatibility.md`：FTP 非 first-class；外置优先；compat 指向本清单/草案  
- [ ] （可选）README.zh-CN / README 存储小节一句：WebDAV 含 OpenList 等代理；FTP 见兼容说明  
- [x] 草案 v2 与清单互相链接已存在  
- [ ] FTP 合并后新增 `docs/storage-ftp.md`（配置项、FTPS、`allow_insecure`、限制、OpenList 对照）

---

## Phase P0 — Caps 元数据 + 策略 UI（三驱动）

### 后端

- [ ] `internal/storage/caps.go`：`Tier`、`Caps`、`Effective`、`CapabilityProvider`、`capsByDriver`  
- [ ] 矩阵与草案 §3 一致：`ListPrefix`/`MultipartUpload` 全 false  
- [ ] `CapsForDriver` / `EffectiveFor(policy)`（或放 `storagesvc`）  
- [ ] 单测：`PrivatePresignCapable` ⇔ 类型实现 `Presigner`（s3 yes，local/webdav no）  
- [ ] `adminPolicyDTO` / list/create/update 响应增加：`tier`、`caps`、`effective`、`warnings`  
- [ ] warnings 计算：`cdn_not_recommended`、`presign_unconfigured`、`insecure_transport`（条件见草案）  
- [ ] `web/src/api/types.ts`：`AdminPolicy` 类型扩展  
- [ ] API/集成测试：policies JSON 含新字段（快照或字段断言）

### 前端

- [ ] i18n `zh`/`en`：tier、caps 面板、loss、warnings、帮助 `ftpPreferProxy`（即使暂无 FTP 驱动也可先放帮助）  
- [ ] `PolicyCapsPanel`（或内联）：展示 tier / summary / 布尔能力 / limitations  
- [ ] `PoliciesPage`：列表徽章；CDN 下 warning；空状态文案  
- [ ] 文案限定：**原图 `/i`**，缩略图不走 CDN/预签名  
- [ ] 组件/页面测试：切换 driver 时 caps 展示变化（mock API）

### doctor

- [ ] `storage_caps` / `cdn_vs_caps` / `compat_only` / `insecure_transport` / `presign_unconfigured`  
- [ ] 与 `cdn_metering` 文案去重  
- [ ] doctor 测试覆盖至少一条 WARN 路径

### P0 完成标准

- [ ] `go test` 相关包通过  
- [ ] 前端 unit 通过  
- [ ] 无 serve 行为变更（仅元数据与展示）

---

## Phase P1 — 校验加固 + compat 审计钩子

- [ ] `cdn_domain` 校验：http(s)、host 必填、禁 userinfo/query/fragment；path 策略单测锁定  
- [ ] Create/Update 非法 `cdn_domain` → 400  
- [ ] 预留 `policy_enable_compat` 审计：当 `tier=compat` 且 enabled 变为 true 时写入（P3 前可用 webdav 模拟或仅代码路径备好）  
- [ ] （可选）保存前前端仍可本地预览 warnings，但以响应为准

---

## Phase P2 — Registry（为 FTP 铺路，可与 P3 同 PR）

- [ ] `storage.Register` 或 `storagesvc` 注册表，包裹现有 switch  
- [ ] local/s3/webdav 启动注册  
- [ ] 未知 driver 错误信息稳定  
- [ ] **不做** Catalog HTTP API（除非同时做动态表单）

---

## Phase P3 — FTP compat 驱动

### 驱动

- [ ] 新建 `internal/storage/ftp`（独立包）  
- [ ] 实现 `Driver`：`Put` / `Open` / `Delete` / `Exists`  
- [ ] 默认 FTPS；`allow_insecure=true` 才允许明文 ftp  
- [ ] config 键草案：`host`/`endpoint`、`port`、`username`、`password`、`prefix`/`root`、`passive`（若需要）、`allow_insecure`  
- [ ] `CapabilityProvider` 或静态表：`tier=compat` + 全套 `feature_loss_keys`  
- [ ] **禁止** 实现仅为了标榜的 List/Multipart  
- [ ] 单元测试：mock FTP 或集成测试说明（CI 可 skip 无服务时）  
- [ ] 密码在 admin DTO 打码（对齐 s3/webdav）

### 接入

- [ ] Resolver/Registry 增加 `ftp`  
- [ ] `validateDriverConfig("ftp")`  
- [ ] `TestPolicy` 写/读/删探针支持 ftp  
- [ ] **rg 验收**：`internal/handler/serve.go` 无 `ftp` 字符串分支  
- [ ] upload/trash/migrate 等仅经 `Driver`，无新特判

### UI

- [ ] 驱动选择器「兼容与传统」→ FTP  
- [ ] FTP 表单字段 + secret 打码提示  
- [ ] 启用 ack 文案 + 调用保存；后端审计 `policy_enable_compat`  
- [ ] Caps 面板展示全部 FTP loss  
- [ ] **不要**「即将推出」半成品入口（仅完整可保存驱动）

### 文档

- [ ] `docs/storage-ftp.md`：双轨决策树、OpenList 优先、限制表、配置示例  
- [ ] 更新 `s3-compatibility.md` Out of scope / Compatibility 节  
- [ ] CHANGELOG 条目：Caps + FTP compat（若分版本发布则分条）

### P3 完成标准

- [ ] 真实或近似供应商路径验证（如 Namecrane / 自建 vsftpd）至少一种  
- [ ] 可删除性：注释删除步骤写在 `storage-ftp.md` 或清单备注  
- [ ] 回归：S3 CDN 302、S3 预签名、WebDAV 流式无行为回退

---

## Phase P4 — 可选硬门禁（另议）

- [ ] 决策：无 `PublicCDNOffloadRecommended` 时禁止 `cdn_domain` 或需 `force_*`  
- [ ] 迁移说明：现网已填 CDN 的 local/webdav 策略  
- [ ] 默认 **不做**，除非误配工单过多

---

## Phase P5 — 远期（本清单不实施）

- [ ] 进程外 storage sidecar  
- [ ] Go plugin `.so` — **明确不做**

---

## 建议 PR 切片

| PR | 内容 | 依赖 |
|----|------|------|
| PR-A | 文档双轨（s3-compatibility + 草案/清单） | 无 |
| PR-B | P0 后端 Caps + DTO + tests | A 可并行 |
| PR-C | P0 前端面板 + i18n | B（或 mock） |
| PR-D | P0 doctor | B |
| PR-E | P1 cdn 校验 + audit 钩子 | B |
| PR-F | P2 Registry | B |
| PR-G | P3 FTP 驱动 + UI + docs | E/F 建议先合 |

最小可发布：仅 **PR-A+B+C+D**（无 FTP 也已改善告知）。  
获客完整：再 **PR-G**。

---

## 验收命令备忘

```bash
# 后端（示例）
go test ./internal/storage/... ./internal/service/storagesvc/... ./internal/service/adminsvc/... ./internal/doctor/...

# 禁止 serve 特判（P3 后）
rg -n 'ftp' internal/handler/serve.go && echo 'FAIL: serve must not mention ftp' || echo 'OK'

# 前端
cd web && npm test -- --run policies
```

（具体测试命令以仓库 Makefile/CI 为准。）

---

## 决策日志

| 日期 | 决定 |
|------|------|
| 2026-07-30 | 外置 OpenList/代理优先；内置 FTP 为 compat 获客路径 |
| 2026-07-30 | P0 后端 warnings；Catalog 非 P0；无「即将推出」UI |
| 2026-07-30 | List/Multipart Caps 在无接口前 false；静态/Effective 分离 |
| 2026-07-30 | Codex 审阅：WATCH；按 v2 收窄后可开工 |
