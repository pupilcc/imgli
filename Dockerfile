# syntax=docker/dockerfile:1
# 默认发行镜像：libvips（WebP 缩略图 + 可选原图转 WebP）。
# 本地纯 Go 开发仍用 make build / make build-go（无需系统 vips）。
FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-fund --no-audit
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
# cgo + pkg-config + vips 头文件（与 -tags vips 配套）
RUN apk add --no-cache build-base pkgconf vips-dev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
# 发布时传入 git tag，例如: docker build --build-arg VERSION=v0.1.0
# 本地/compose 未传时回退为 docker，便于区分非 release 构建。
ARG VERSION=docker
RUN CGO_ENABLED=1 go build -tags vips \
	-ldflags "-X github.com/yixian-huang/imgli/internal/version.Version=${VERSION}" \
	-o /imgli ./cmd/imgli

FROM alpine:3.20
# 运行时需要 libvips 动态库（与构建期 vips-dev 对应）
RUN apk add --no-cache ca-certificates tzdata vips \
	&& adduser -D -u 1000 imgli \
	&& mkdir -p /data && chown imgli /data
COPY --from=build /imgli /usr/local/bin/imgli
USER imgli
ENV IMGLI_LISTEN=:8686 IMGLI_DATA_DIR=/data
VOLUME /data
EXPOSE 8686
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
	CMD wget -qO- http://127.0.0.1:8686/healthz || exit 1
ENTRYPOINT ["imgli"]
CMD ["serve"]
