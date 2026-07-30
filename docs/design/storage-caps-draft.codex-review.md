**Review**

**CRITICAL**

None.

**IMPORTANT**

1. [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:231): "`list_prefix` | true（API 有，产品未暴露）" and [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:232): "`multipart_upload` | true（SDK 有，产品未暴露）" are not accurate for the current codebase. The actual storage contract is only `Put/Open/Delete/Exists` in [storage.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/storage/storage.go:14), and the local/S3/WebDAV drivers only implement those methods in [local.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/storage/local/local.go:40), [driver.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/storage/s3/driver.go:17), and [driver.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/storage/webdav/driver.go:152). Recommendation: mark these `false` in P0, or introduce real optional interfaces before advertising them.

2. [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:67): "`能力描述‘驱动能否’，不是‘策略是否已配置’`" conflicts with fields that are inherently policy-config dependent, especially `transport_tls` and effective presign availability. S3 accepts `http://` endpoints in [driver.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/storage/s3/driver.go:39), WebDAV accepts both `http` and `https` in [policies.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/service/adminsvc/policies.go:121), and S3 `PresignGet` only works when `presign_domain` is configured in [presign.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/storage/s3/presign.go:40). Recommendation: split static driver caps from effective per-policy state, or rename these fields so the API does not imply runtime truth it cannot guarantee.

3. [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:184): "`API 返回 warnings[]`" and [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:215): "`新建/更新响应同样可带 warnings`" do not align with the phase table and acceptance criteria, which defer write warnings to P1 and allow frontend-local inference in P0 at [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:465) and [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:498). Current DTOs also have no `caps/tier/warnings` in [admin_policies.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/handler/admin_policies.go:40) and [types.ts](/Users/yixian.huang/ai/yixian-content/imgli/web/src/api/types.ts:313). Recommendation: make backend-authored `caps/tier/warnings` part of P0 for list/create/update, otherwise UI and `doctor` will fork logic.

4. [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:346): "`填写后公开图可能 302 到该前缀`" is directionally right, but the doc does not define `cdn_domain` validation at all. Current backend accepts raw `cdn_domain` in [admin_policies.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/handler/admin_policies.go:75) / [policies.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/service/adminsvc/policies.go:284), and `ObjectURL` concatenates it directly in [storagesvc.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/service/storagesvc/storagesvc.go:143). Recommendation: specify validation now: `http(s)` only, host required, no userinfo/query/fragment, and explicitly decide whether path prefixes are allowed.

5. [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:233): "`public_cdn_offload` | false" for local/WebDAV is semantically muddy. Current public-original redirects are driver-agnostic in [serve.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/handler/serve.go:222) and [storagesvc.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/service/storagesvc/storagesvc.go:143); a local policy can still 302 if `cdn_domain` is set. Recommendation: either split this into `redirect_supported` vs `recommended_origin`, or keep `false` but render it as “not recommended / not guaranteed”, not “unsupported”.

6. [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:447): the `doctor` section is incomplete relative to current behavior. `doctor` only has coarse `cdn_metering` checks today in [doctor.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/doctor/doctor.go:79), and the draft does not specify what happens for configured-but-broken presign or non-TLS remote endpoints. Recommendation: add explicit checks for configured HTTP remote storage and configured presign that is expected but inoperable, or narrow the P0 `doctor` scope.

7. [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:467): "`P3 ftp compat`" conflicts with current public docs that say FTP as a driver is “not planned” in [s3-compatibility.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/s3-compatibility.md:49). Recommendation: resolve the roadmap before landing this draft, or remove FTP-specific rollout/UI promises and keep the design generic to future compat drivers.

8. [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:496): "`GET /admin/policies` 每项含 tier + caps`" is incomplete against the actual route shape, which is `/api/v1/admin/policies` in [admin_policies.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/handler/admin_policies.go:55). Recommendation: use the real API path consistently in acceptance criteria.

**SUGGESTION**

1. [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:121): `Capabiliator` is a typo. Recommendation: rename to something like `CapabilityProvider`.

2. [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:341): the `cdn_domain` interaction text would be clearer if it explicitly said these capabilities apply to original `/i` delivery, not thumbnails. Thumbnails are always app-served today in [serve.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/handler/serve.go:274) and do not redirect through CDN or presign paths. Recommendation: scope the copy to original-image delivery.

3. [storage-caps-draft.md](/Users/yixian.huang/ai/yixian-content/imgli/docs/design/storage-caps-draft.md:464): P0 should stay “metadata only”. The architect review concern is valid: resolver switch, admin validation switch, UI driver list, and docs are already separate truth sources in [storagesvc.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/service/storagesvc/storagesvc.go:71), [policies.go](/Users/yixian.huang/ai/yixian-content/imgli/internal/service/adminsvc/policies.go:131), and [PoliciesPage.tsx](/Users/yixian.huang/ai/yixian-content/imgli/web/src/pages/admin/policies/PoliciesPage.tsx:233). Recommendation: keep `Caps` advisory in P0, and defer `Catalog`/`Register` until there is a real registry-backed driver path.

**Open Questions**

1. `local/webdav public_cdn_offload`: choose `false` plus warning. That best matches current code: redirect may happen, but product-level anonymous-read/CDN-origin suitability is not guaranteed.

2. `warnings` vs frontend-only derivation: backend `warnings` should be P0. The server should be the canonical rule source; the frontend can mirror hints for unsaved forms.

3. `Catalog API`: not P0. Keep the current hardcoded `local|s3|webdav` UI until a registry actually exists.

4. `compat` enable ack audit: yes. Add a dedicated audit event like `policy_enable_compat`; generic `policy_update` is too weak for explicit risk acceptance.

5. FTP before implementation: do not show “即将推出”. It conflicts with current docs and turns the UI into a roadmap promise. If needed, use help text saying “not supported yet; prefer S3-compatible/WebDAV or migration workflows.”

**Overall**

The draft is directionally good, but it is not ready as written. The main issues are correctness of the capability matrix, mixing static capability with effective configured state, unresolved backend contract for warnings, and the FTP roadmap conflict. Architecturally this is a `WATCH`: proceed with a narrower P0 where `Caps` is advisory metadata, not a new control plane.