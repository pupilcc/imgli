#!/bin/sh
# Docker entrypoint: fix common bind-mount permission issues then drop privileges.
# Official image runs as uid 1000 (user imgli). Host bind mounts are often root-owned.
set -e

DATA_DIR="${IMGLI_DATA_DIR:-/data}"

if [ "$(id -u)" = "0" ]; then
	mkdir -p "$DATA_DIR" 2>/dev/null || true
	# Non-recursive: enough for first-run create of imgli.db / uploads; avoids slow chown -R.
	chown imgli:imgli "$DATA_DIR" 2>/dev/null || true
	# Fix default SQLite files if present and not writable by imgli.
	for f in "$DATA_DIR/imgli.db" "$DATA_DIR/imgli.db-wal" "$DATA_DIR/imgli.db-shm"; do
		if [ -e "$f" ]; then
			chown imgli:imgli "$f" 2>/dev/null || true
		fi
	done
	exec su-exec imgli /usr/local/bin/imgli "$@"
fi

exec /usr/local/bin/imgli "$@"
