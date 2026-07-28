# imgli OCR sidecar (PP-OCR)

图片文字识别旁路服务,供 imgli 的 `ocr_keywords` 机审插件调用。
HTTP 契约与 `moderation.OCRKeywordsChecker` 一致:`POST` 原图(Bearer Token)→ `{"text":"..."}`,`GET /health` → `ok-ocr`。

| 文件 | 用途 |
|------|------|
| `server_rapidocr.py` / `Dockerfile.rapid` | RapidOCR + onnxruntime(PP-OCR 模型,体积小) |
| `server.py` / `Dockerfile` | 原生 paddlepaddle 3.3.1 + paddleocr 3.x(体积更大) |
| `server_pocr29.py` / `Dockerfile.p30` | paddlepaddle 3.0.0 + paddleocr 2.9.1(规避 3.3.x oneDNN/PIR 问题) |

## 部署示例

```bash
docker build -f Dockerfile.rapid -t imgli-ocr .
docker run -d --name imgli-ocr --restart unless-stopped --network host \
  -e TOKEN='<随机长 token>' -e PORT=3199 -e HOST=0.0.0.0 \
  -e ALLOW_IPS=203.0.113.10,127.0.0.1 \
  imgli-ocr
```

- **访问控制**:`ALLOW_IPS`(逗号分隔,仅白名单 IP 可访问,含 `/health`)+ `TOKEN`(Bearer,POST 必需)。两者都设,单靠 Token 不足以挡探测。
- 建议再加云防火墙纵深:仅放行 OCR 端口给调用方 IP。
- 在 imgli Admin → 系统设置 → OCR 词表中填 endpoint(`http://<host>:3199/`)与 api_key(=TOKEN)。

## 已知坑

- paddle 3.3.x 的 predict(oneDNN/PIR)在部分 CPU 上不稳,用 `Dockerfile.p30` 组合规避;Zen5 上 paddle 2.6 import 直接 segfault。
- 容器 entrypoint 必须是 `python3 /app/server.py`;勿把带 `sleep` 的临时调试容器 commit 成运行镜像。
- `ops-patch-allow-ips.py` / `ops-cutover-allow-ips.sh`:给存量旧镜像热补 `ALLOW_IPS` 支持的运维辅助脚本,新部署用不到。
