# 阶段 0：`i.img.li` Cloudflare 缓存 + curl 验收清单

面向 **img.li 生产**：公开原图 `img.li/i/...` → **302** → `https://i.img.li/{object}` → **CF 橙云** → Hosthatch RustFS。

| 项 | 值 |
|----|-----|
| 媒体 CDN 域 | `i.img.li`（**必须** Proxied / 橙云） |
| 门禁域 | `img.li`（302 发出方；本清单不要求缓存 302） |
| 预签名域 | `s3.img.li`（灰云；**不要**当公开缓存加速对象） |
| 源站 Cache-Control（openresty） | `public, max-age=2592000`（30d），见同目录 `i.img.li.conf` |
| 策略 | 先调 CF + 观测；**不对** `/i` 的 302 做长缓存 |

勾选框：`[ ]` → 完成后改 `[x]`。每步失败先记现象再改下一项。

---

## A. 控制台前置（DNS / SSL）

- [ ] **A1** Cloudflare 区域中存在 `i.img.li`（CNAME 或 A 到源；与现网一致即可）
- [ ] **A2** `i.img.li` 代理状态为 **Proxied（橙云）**（灰云 = 无边缘缓存，阶段 0 失败）
- [ ] **A3** SSL/TLS 模式与源站证书匹配（常见 **Full** 或 **Full (strict)**；源已 HTTPS 时优先 strict）
- [ ] **A4** 确认 **Always Use HTTPS** 开启（或等价跳转），避免混用 http 缓存分裂
- [ ] **A5**（可选）HTTP/3 (QUIC) 开启
- [ ] **A6** `s3.img.li` 仍为 **DNS only（灰云）**——私密预签名域，不要改成橙云「加速」

**DNS 快速核：**

```bash
# 橙云时通常看到 Cloudflare anycast IP，而非仅源站 IP
dig +short i.img.li A
dig +short i.img.li AAAA
```

- [ ] **A7** 记录当前解析结果（粘贴到运维笔记），便于回滚对照

### A8. 用 `npc cf` 探 DNS（推荐，比 dig 准）

本机已配置 NoPanel Cloudflare token 时：

```bash
npc cf auth-status -o json
npc cf token-check -o json          # 看 dns_read / cache_purge / ssl_read 等

# 图床生产区
npc cf dns-list --zone img.li -o json

# 产品站区（与媒体 CDN 分离）
npc cf dns-list --zone imgli.com -o json
```

**期望（img.li 生产，2026-07-29 `npc cf` 实扫）：**

| name | type | proxied | content |
|------|------|---------|---------|
| `i.img.li` | A | **true（橙云）** | Hosthatch `103.214.22.62` |
| `img.li` | A | **false（灰云）** | VIP `82.158.226.66` |
| `s3.img.li` | A | **false（灰云）** | Hosthatch `103.214.22.62` |

**imgli.com（产品站，非图床对象 CDN）：**

| name | type | proxied | content |
|------|------|---------|---------|
| `imgli.com` | A | true | `103.73.220.161` |
| `www.imgli.com` | A | true | `103.73.220.161` |

- [ ] **A8.1** `npc cf dns-list --zone img.li` 中 `i.img.li` **proxied=true**
- [ ] **A8.2** `img.li` apex 与 `s3.img.li` **proxied=false**
- [ ] **A8.3**（可选）`imgli.com` 记录符合产品站预期；**勿**把图床对象误指到 imgli.com
- [ ] **A8.4** 知悉：`ssl-status` 可能 403（token 无 `ssl_read`）→ SSL 模式仍要控制台看
- [ ] **A8.5** 知悉：Cache Rules **不能**经当前 `npc cf` 列出 → 规则细节仍要控制台；**HIT 用 curl**

---

## B. Cache Rules（推荐：Caching → Cache Rules）

> 旧「Page Rules」可迁到 Cache Rules。以下用 **规则逻辑** 描述，UI 文案随 CF 改版可能略有出入。

### B1. 规则：缓存公开媒体对象（主规则）

- [ ] **B1.1** 新建 Cache Rule，名称建议：`i-img-li-media-cache`
- [ ] **B1.2** 匹配条件（满足其一即可，按你控制台支持的字段选）：

**推荐（主机 + 扩展名）：**

```text
(http.host eq "i.img.li" and (
  http.request.uri.path.extension in {"png" "jpg" "jpeg" "webp" "gif" "avif" "svg" "ico" "bmp"}
))
```

**若 extension 字段不可用，用路径后缀（示例）：**

```text
(http.host eq "i.img.li" and (
  ends_with(http.request.uri.path, ".png") or
  ends_with(http.request.uri.path, ".jpg") or
  ends_with(http.request.uri.path, ".jpeg") or
  ends_with(http.request.uri.path, ".webp") or
  ends_with(http.request.uri.path, ".gif") or
  ends_with(http.request.uri.path, ".avif")
))
```

- [ ] **B1.3** 缓存资格：**Eligible for cache**（可缓存）
- [ ] **B1.4** Edge TTL：
  - 优先 **Respect origin**（源站已 `max-age=2592000`），或
  - 显式 **30 days**（与源站对齐）
- [ ] **B1.5** Browser TTL：与 Edge 同量级或略短（如 1d–30d）；不要设成「几分钟」抵消源站 30d
- [ ] **B1.6** Cache key：默认即可；**无**额外把 cookie 进 key（媒体域不应依赖 cookie）
- [ ] **B1.7** 查询串：对象 URL 一般无 `?v=` → **Ignore query string**（或 cache key 不含 query），提高命中
- [ ] **B1.8** 保存并 **Deploy**；确认规则顺序：更具体的规则在前（若有多条）

### B2. 规则：根路径 / 非对象不瞎缓存（可选但推荐）

- [ ] **B2.1** 新建规则 `i-img-li-bypass-root`（或并入例外）
- [ ] **B2.2** 匹配：`http.host eq "i.img.li" and http.request.uri.path eq "/"`
- [ ] **B2.3** 动作：**Bypass cache**（源站根路径为 404 字面量，无需缓存）

### B3. 不要做的规则

- [ ] **B3.1** **没有**对 `img.li/i/*` 的 302 响应做「Cache Everything + 长 TTL」
- [ ] **B3.2** **没有**对 `s3.img.li` 开启橙云 + 长缓存
- [ ] **B3.3** **没有**对 `/api` 或带会话的 HTML 全局 Cache Everything

### B4. 配置级增强（账号能力内）

- [ ] **B4.1** Caching → **Tiered Cache** 已开（有则开）
- [ ] **B4.2** 确认无全局「Development Mode」长期开着（会绕过缓存）
- [ ] **B4.3**（可选）Caching → Configuration：**Caching Level = Standard**
- [ ] **B4.4**（可选）Purge 权限谁可用：仅运维；删图/误缓存时用 **Custom Purge URL**

---

## C. 源站侧核对（Hosthatch openresty）

与仓库 `deploy/ops/i.img.li.conf` 对齐：

- [ ] **C1** 生产 conf 含类似：

  ```nginx
  expires 30d;
  add_header Cache-Control "public, max-age=2592000" always;
  ```

- [ ] **C2** `location = /` 仍 404 短文本（勿对 `/` 返回可被误缓存的大页面）
- [ ] **C3** 回源 `proxy_pass` 到桶前缀正确（现网：`/imgli/`）
- [ ] **C4**（安全）桶仍为 **不可 ListBucket** 的 GetObject 策略（见运维 KB）；加速不得靠打开 List

**源站直打（在源机或经内网，可选）：**

```bash
# 若可访问源站回源路径，应看到 Cache-Control: public, max-age=2592000
# 公网请优先用下方 D/E 经 CF 的验收
```

- [ ] **C5** 记：源站头与 CF「Respect origin」一致，避免 Edge TTL=2 分钟之类默认盖掉 30d

---

## D. curl 验收 — 准备样例

准备 **一张确定公开、不会马上删除** 的图：

```bash
# 门禁链（复制链形态）
GATE='https://img.li/i/__________.png'   # ← 换成真实 key

# 从 302 解析媒体 URL（或从管理后台/DB 已知 object path）
MEDIA=$(curl -sI -o /dev/null -w '%{redirect_url}' "$GATE")
# 部分 curl 对 302 要用 -D 解析 Location：
MEDIA=$(curl -s -D - -o /dev/null "$GATE" | awk 'tolower($1)=="location:"{print $2}' | tr -d '\r')
echo "GATE=$GATE"
echo "MEDIA=$MEDIA"
```

- [ ] **D1** `GATE` 返回 **302**（不是 200 流式整图；不是 401/404）
- [ ] **D2** `Location` 主机为 **`i.img.li`**，路径为对象键（可含 `public/` 或日期前缀）
- [ ] **D3** 将 `MEDIA` 固定进环境，后续命令复用

**注意：** 对 `/i` 使用 **GET** 看状态（`curl -I` / HEAD 可能 **405**）。推荐：

```bash
curl -s -D - -o /dev/null "$GATE" | head -n 20
```

---

## E. curl 验收 — 缓存命中（核心）

### E1. 响应头基线（第一次可能 MISS）

```bash
curl -sI "$MEDIA" | tee /tmp/i-img-li-headers-1.txt
```

检查并勾选：

- [ ] **E1.1** HTTP **200**
- [ ] **E1.2** 存在 **`CF-Cache-Status`**（无此头 ≈ 未走橙云或未到 CF 缓存层）
- [ ] **E1.3** 第一次允许：`MISS` / `EXPIRED` / `MISS, ...`；记录：`________`
- [ ] **E1.4** 存在缓存相关头之一：`Cache-Control: public` 且 `max-age` 较大，或 CF 年龄头
- [ ] **E1.5** `cf-ray` 存在（证明经 CF）

### E2. 第二次应 HIT

```bash
sleep 1
curl -sI "$MEDIA" | tee /tmp/i-img-li-headers-2.txt
grep -i cf-cache-status /tmp/i-img-li-headers-2.txt
```

- [ ] **E2.1** `CF-Cache-Status: **HIT**`（或 `REVALIDATED` 且后续稳定 HIT）
- [ ] **E2.2** 若仍为 `DYNAMIC` / `BYPASS` / `NONE`：回到 B 节规则与 Development Mode
- [ ] **E2.3** 若为 `MISS` 反复：查规则是否未 Deploy、主机不匹配、扩展名未覆盖、被 cookie 等拆 key

### E3. 体下载与耗时（粗测）

```bash
curl -sL -o /tmp/t1.bin -w 'code=%{http_code} size=%{size_download} time=%{time_total}\n' "$MEDIA"
curl -sL -o /tmp/t2.bin -w 'code=%{http_code} size=%{size_download} time=%{time_total}\n' "$MEDIA"
cmp /tmp/t1.bin /tmp/t2.bin && echo 'bytes match'
```

- [ ] **E3.1** 两次均为 **200**，size 一致
- [ ] **E3.2** 第二次 `time_total` 通常更短或不差于第一次（弱网下仅作参考）
- [ ] **E3.3** 文件可打开为正常图片（`file /tmp/t1.bin`）

### E4. 门禁链端到端

```bash
curl -sL -o /tmp/via-gate.bin -w 'code=%{http_code} size=%{size_download} time=%{time_total}\n' "$GATE"
```

- [ ] **E4.1** 跟随重定向后 **200**，图完整
- [ ] **E4.2** 仅查 302、不跟随：

```bash
curl -s -D - -o /dev/null "$GATE" | head -n 15
# 期望：HTTP/2 302 且 Location: https://i.img.li/...
```

### E5. 负向：不该被「加速坏掉」的路径

```bash
# 根路径
curl -s -D - -o /dev/null "https://i.img.li/" | head -n 15

# 随机不存在对象（键需像真路径时按你们实际 404 行为）
curl -s -D - -o /dev/null "https://i.img.li/does-not-exist-$(date +%s).png" | head -n 15
```

- [ ] **E5.1** `https://i.img.li/` → **404**（或源站短文本），且不宜被长期当成 200 缓存首页
- [ ] **E5.2** 不存在对象 → **404**（或源站等价）；注意：**404 若被长缓存**，补传同 URL 会短期仍 404——不可变随机键下风险低；固定 key 重传需知悉
- [ ] **E5.3** 私密图：**不得**出现「无登录即可稳定 200 + CF HIT 长缓存」的验收标准（私密应走门禁/预签名，不进本清单 HIT 目标）

---

## F. 观测与告警（阶段 0 收尾）

- [ ] **F1** CF → Analytics → **Traffic / Cache**：能看到 `i.img.li` 请求与 Cache 比例
- [ ] **F2** 记录基线（日期 + 截图或数字）：

  | 指标 | 值 | 日期 |
  |------|-----|------|
  | 缓存命中率（约） | | |
  | 回源带宽（约） | | |
  | 边缘带宽（约） | | |

- [ ] **F3** 账单/用量告警：CF 与（若有）源站流量超预算邮件（对齐早期成本纪律）
- [ ] **F4** 确认 **Development Mode 关闭**
- [ ] **F5** 运维笔记写明：Purge 入口与「删图后最多缓存 30d 或手动 Purge」

### F6. `npc cf cache-purge`（改源站头 / 误缓存后）

语法（`npc cf cache-purge` 无子帮助时以 usage 为准）：

```bash
# 单 URL（推荐：只清一张图）
npc cf cache-purge --zone img.li --url 'https://i.img.li/public/2026/07/19/example.png'

# 整区（慎用：瞬时回源暴涨）
npc cf cache-purge --zone img.li --all
```

- [ ] **F6.1** 需要时用 **--url** 精确 purge，避免无必要 `--all`
- [ ] **F6.2** purge 后对该 URL：`curl -sI` 可能先 **MISS**，再请求应 **HIT**
- [ ] **F6.3** token 具备 cache_purge（`npc cf token-check` 中 pass）

**产品站 purge（与图床分离）：**

```bash
npc cf cache-purge --zone imgli.com --url 'https://imgli.com/'
# 或 --all（同样慎用）
```

---

## G. 故障速查

| 现象 | 可能原因 | 处理 |
|------|----------|------|
| 无 `CF-Cache-Status` | 灰云 / 未走 CF | A2 Proxied |
| 一直 `DYNAMIC` | 规则未匹配或源标不可缓存 | B1 规则、源 `Cache-Control` |
| 一直 `BYPASS` | Bypass 规则 / dev mode / 特殊 cookie | B4.2、规则顺序 |
| 一直 `MISS` | TTL=0、缓存资格关闭、每次 key 不同 | B1.3–B1.7 |
| `img.li/i` 200 大图不 302 | 策略无 `cdn_domain` 或非公开 | Admin 存储策略 / 可见性 |
| HIT 但图是旧的 | 长缓存 + 同 URL 覆盖写 | 图床宜不可变键；必要 Purge |
| 加速后流量暴涨 | 盗链 | CDN Referer + 配额（成本页） |
| `npc cf ssl-status` 403 | token 无 ssl_read | 控制台看 SSL；或扩 token 权限 |
| DNS 已橙云但无 HIT | 规则/Dev Mode/源头 | B + F4；curl 看 `CF-Cache-Status` |
| 清了还是旧图 | purge 错 zone/URL | 确认 zone=`img.li` 且 URL 为 **i.img.li** 对象地址 |

### 公网 + npc 联合探查记录（2026-07-29）

| 检查 | 结果 |
|------|------|
| `npc cf` DNS `i.img.li` proxied | **true** |
| `img.li` / `s3.img.li` proxied | **false** |
| curl 302 → `i.img.li` | **OK** |
| curl `CF-Cache-Status` 第二次 | **HIT**（样例对象） |
| 源 `Cache-Control` | **public, max-age=2592000** |
| `npc cf ssl-status` | **403**（权限不足） |
| Cache Rules via npc | **不可列** |

---

## H. 阶段 0 完成定义（全部满足才勾）

- [ ] **H1** `i.img.li` 橙云 + 媒体 Cache Rule 已 Deploy  
- [ ] **H2** 同一 `MEDIA` URL 连续请求出现 **`CF-Cache-Status: HIT`**  
- [ ] **H3** `GATE` 仍为 **302 → i.img.li**，跟随下载 200  
- [ ] **H4** 未对门禁 302 / 私密域做错误长缓存  
- [ ] **H5** 基线指标已记录 + 预算告警已设（或显式接受风险并记录）  
- [ ] **H6** 本清单勾选结果归档（日期、操作者、样例 `GATE`/`MEDIA` 可打码 key）

**阶段 0 完成后下一步（勿提前摊大饼）：**

1. 观察 3–7 天 HIT 率与账单  
2. 再考虑 `/t` 边缘缓存或缩略图对象化 302（阶段 1）  
3. R2 验证与迁源仍属阶段 2  

---

## 相关文档

| 文档 | 说明 |
|------|------|
| `deploy/ops/i.img.li.conf` | 源站反代 + 30d Cache-Control |
| omni `imgli/ops-storage-s3-cdn` | 生产拓扑真源 |
| omni `imgli/global-image-acceleration-via-302-cdn-2026-07-29` | 加速原则与阶段路线 |
| omni `imgli/early-stage-ops-and-cost-control-2026-07-29` | 成本与防盗链纪律 |
| omni `imgli/ops-cloudflare-i-img-li-cache-checklist` | 本清单 KB 副本 |

**探查工具分工：** `npc cf dns-list` / `cache-purge` / `token-check` = 配置与权限；`curl` = 运行时 HIT；控制台 = SSL、Cache Rules、Analytics。
