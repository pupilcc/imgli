# ShareX → imgli / img.li

Configure [ShareX](https://getsharex.com/) (Windows) custom uploader to post screenshots to imgli.

**Prerequisites**

1. Running instance (e.g. `https://img.li` or your self-host).
2. Logged-in user → **Settings → API Token** → create token (`upload` or `full`). **Copy the plaintext once.**

Guest upload without a token is rate-limited and not suitable as a permanent ShareX destination.

---

## 1. API contract (same as PicGo)

| Item | Value |
|------|--------|
| Request URL | `https://img.li/api/v1/upload` |
| Method | `POST` |
| Body | Multipart form data |
| File form name | **`file`** |
| Header | `Authorization: Bearer <API_TOKEN>` |
| Optional form field | `visibility` = `public` \| `private` |
| Response URL | JSON path **`data.links.url`** |

Quick check with curl (optional):

```bash
curl -sS -X POST 'https://img.li/api/v1/upload' \
  -H "Authorization: Bearer $IMGLI_TOKEN" \
  -F 'file=@test.png' -F 'visibility=public' | jq .data.links.url
```

---

## 2. Import the sample custom uploader

1. Copy [imgli.sxcu.example](imgli.sxcu.example) and rename to e.g. `imgli.sxcu`.
2. Edit the file: replace `YOUR_TOKEN_HERE` and, if needed, `https://img.li` with your base URL.
3. Double-click the `.sxcu` (or ShareX → **Destinations → Custom uploader settings → Import**).
4. ShareX → **Destinations → Destination settings → Image uploader** → choose **imgli** (or the name in the file).
5. Capture / upload a test image; clipboard should contain `https://…/i/….png`.

### Example `.sxcu` (ShareX 15+ style)

```json
{
  "Version": "15.0.0",
  "Name": "imgli",
  "DestinationType": "ImageUploader",
  "RequestMethod": "POST",
  "RequestURL": "https://img.li/api/v1/upload",
  "Headers": {
    "Authorization": "Bearer YOUR_TOKEN_HERE"
  },
  "Body": "MultipartFormData",
  "FileFormName": "file",
  "Arguments": {
    "visibility": "public"
  },
  "URL": "{json:data.links.url}",
  "ThumbnailURL": "{json:data.links.thumbnail_url}",
  "DeletionURL": "",
  "ErrorMessage": "{json:message}"
}
```

Notes:

- ShareX JSON syntax uses `{json:data.links.url}` (not a separate “JSON path” field).
- Older ShareX builds may show slightly different UI labels; the request shape must stay: **POST multipart `file` + Bearer header**.
- Do **not** commit real tokens into git; keep only the `.example` file in the repo.

---

## 3. Manual setup (without importing)

**Destinations → Custom uploader settings → New**

| Field | Value |
|-------|--------|
| Name | `imgli` |
| Request method | POST |
| Request URL | `https://img.li/api/v1/upload` |
| Body | Multipart form data |
| File form name | `file` |
| Headers | Name `Authorization`, Value `Bearer <token>` |
| Arguments (optional) | Name `visibility`, Value `public` |
| URL | `{json:data.links.url}` |
| Thumbnail URL (optional) | `{json:data.links.thumbnail_url}` |
| Error message (optional) | `{json:message}` |

Save → set as default image uploader → test upload.

---

## 4. Errors

| HTTP | Meaning |
|------|---------|
| 401 | Missing/invalid Bearer token |
| 403 | Guest upload disabled and not authenticated |
| 413 | Over group max file size or storage quota |
| 415 | Extension not allowed |
| 400 | Invalid options (e.g. `expires_over_group` / `max_views_over_group` when the token’s group forbids permanent or over-cap values) |
| 429 | Rate limited or monthly bandwidth cap |

Group limits: `GET /api/v1/user/quota` (authenticated) or `GET /api/v1/config` → `guest`. See [user-groups-lifecycle.md](../user-groups-lifecycle.md).

---

## 5. Related

- [integrations index](README.md)
- [PicGo](../picgo.md) · [uPic](upic.md)
- CLI: `imgli upload` (see repository README)
