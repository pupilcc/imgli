# 压测脚本 `loadtest.py`

stdlib only，无第三方依赖。

## 读压

```bash
# 公网
python3 scripts/loadtest.py suite --base https://img.li --image /i/<key>.png --n 50 --c 10
python3 scripts/loadtest.py read --base https://img.li --path /api/v1/config --n 100 --c 20

# VIP 源站（SSH/npc 上）
python3 scripts/loadtest.py read --base http://127.0.0.1:8686 --path / --n 200 --c 50
python3 scripts/loadtest.py read --base http://127.0.0.1:8686 --path /i/<key>.png --n 200 --c 50
```

注意：`/api/v1/config` 默认约 **60 次/分钟/IP**，高并发会出现 429（预期）。

## 写压（游客或 Token）

```bash
# 游客（生产默认 3/日，压测前需临时提高游客组 rate_*）
python3 scripts/loadtest.py write --base http://127.0.0.1:8686 --n 40 --c 8

# 登录用户
IMGLI_TOKEN=... python3 scripts/loadtest.py write --base https://img.li --n 20 --c 4
```

每张图用不同 seed 生成小 PNG，避免秒传短路。

## 生产游客限速临时抬高（测完务必改回）

```sql
-- 备份当前值后：
UPDATE user_groups SET rate_per_minute=120, rate_per_hour=2000, rate_per_day=5000
WHERE is_guest=1;

-- 测完改回：
UPDATE user_groups SET rate_per_minute=3, rate_per_hour=3, rate_per_day=3
WHERE is_guest=1;
```

限速中间件每次请求读库，**无需重启** baili。
