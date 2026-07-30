# FTP storage (compatibility tier)

imgli can store objects on **FTP/FTPS** as a **compatibility** driver. This is
**not** equivalent to S3 (no private presign, CDN offload not recommended, not
for hot traffic).

## Prefer external proxy first

If the vendor only offers FTP:

1. **OpenList** (or similar): mount FTP → expose **WebDAV** → imgli **webdav** policy  
2. **rclone**: sync/mount to local disk or S3 → imgli **local** / **s3**  
3. Only if you refuse an extra process: use built-in **ftp** driver

See also [s3-compatibility.md](s3-compatibility.md) and the design notes under
`docs/design/storage-caps-*.md`.

## Config keys (admin policy `config` JSON)

| Key | Required | Notes |
|-----|----------|--------|
| `host` | yes* | FTP host (*or `endpoint` without scheme) |
| `port` | no | Default `21` |
| `username` | no | Default anonymous |
| `password` | no | Masked in admin API |
| `prefix` | no | Remote base path |
| `allow_insecure` | no | `true` = plain FTP; default FTPS (TLS) |
| `disable_epsv` | no | `true` for some broken NATs |

## Behaviour

- Implements `Put` / `Open` / `Delete` / `Exists` only  
- **No** special cases in the image serve path (`/i` still streams when no CDN/presign)  
- Admin **Test connection** runs write/read/delete probe  
- Enabling an enabled compat policy records audit `policy_enable_compat`  

## Removal

Delete package `internal/storage/ftp`, the `ftp` branches in
`storagesvc` / `adminsvc` / admin UI, and the Caps table entry for `ftp`.
