# uPic / PicList → imgli / img.li

Configure [uPic](https://github.com/gee1k/uPic) (macOS) or similar **custom HTTP** uploaders (e.g. [PicList](https://github.com/Kuingsmile/PicList) custom) against imgli’s upload API.

**Prerequisites**

1. Instance base URL (public: `https://img.li`, or your self-host).
2. **Settings → API Token** → create token; copy plaintext once.

---

## 1. Shared contract

| Item | Value |
|------|--------|
| URL | `{BASE}/api/v1/upload` |
| Method | `POST` |
| Content-Type | `multipart/form-data` (client-generated) |
| File field | **`file`** |
| Header | `Authorization: Bearer <API_TOKEN>` |
| Optional form | `visibility` = `public` \| `private` |
| URL extraction | **`data.links.url`** |

This is the same mapping as [PicGo](../picgo.md) and [ShareX](sharex.md).

```bash
export IMGLI_TOKEN='…'
curl -sS -X POST 'https://img.li/api/v1/upload' \
  -H "Authorization: Bearer $IMGLI_TOKEN" \
  -F 'file=@test.png' | jq .data.links.url
```

---

## 2. uPic (Custom)

UI labels vary by uPic version; map fields as follows.

1. Open **uPic → Preferences → Host** (图床) → add **Custom** / 自定义.
2. Fill:

| uPic field (typical) | Value |
|----------------------|--------|
| API URL / 上传地址 | `https://img.li/api/v1/upload` |
| Method | `POST` |
| File field / 文件字段名 | `file` |
| URL path / 返回值路径 | `data.links.url` （部分版本写 `.data.links.url`） |
| Headers | Key `Authorization`，Value `Bearer <token>`（整段，注意空格） |
| Other form fields (optional) | `visibility` = `public` |

3. Save → **Test** / 上传测试图 → confirm the returned URL opens in a browser.
4. Set this host as default for screenshot / clipboard upload.

### Tips

- If the client only supports “raw JSON body” uploaders, it **cannot** match imgli (we require **multipart `file`**). Use Custom/Web-style hosts only.
- Token in headers: `Bearer ` + token (one space). Wrong scheme → HTTP 401.
- Self-host: replace host only; path stays `/api/v1/upload`.

---

## 3. PicList / other PicGo-compatible clients

PicList and many PicGo forks accept the same **web-uploader** shape as [picgo.md](../picgo.md):

```json
{
  "picBed": {
    "uploader": "web-uploader",
    "web-uploader": {
      "url": "https://img.li/api/v1/upload",
      "paramName": "file",
      "jsonPath": "data.links.url",
      "customHeader": "{\"Authorization\":\"Bearer YOUR_TOKEN_HERE\"}"
    }
  }
}
```

Plugin package names differ (`web-uploader` vs market variants); after curl works, only copy URL / header / JSON path into the plugin.

---

## 4. Errors

| HTTP | Meaning |
|------|---------|
| 401 | Bad or missing Bearer token |
| 403 | Guest closed / not allowed |
| 400 | Invalid options (e.g. group forbids permanent / over-cap `expires_in` → `expires_over_group`) |
| 413 | File too large or quota |
| 415 | Extension not allowed |
| 429 | Rate limit or bandwidth cap |

Group limits: `GET /api/v1/user/quota`. See [user-groups-lifecycle.md](../user-groups-lifecycle.md).

---

## 5. Related

- [integrations index](README.md)
- [ShareX](sharex.md) · [PicGo](../picgo.md)
- CLI: `imgli upload -verbose` (repository README)
