# 运维手册

## 日常运维

### 查看状态

```bash
systemctl status ip-lookup
journalctl -u ip-lookup -f --since "5 min ago"
```

### 日志

- 路径：`/var/log/ip-lookup/ip-lookup.log`
- 格式：JSON 结构化
- 轮转：50MB 文件大小，保留 7 份，30 天
- 级别：`debug/info/warn/error`（配置项 `log_level`）

### 指标

```bash
curl localhost:20013/metrics
```

指标清单：

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `http_requests_total` | counter | 总请求数 |
| `rate_limit_hits_total` | counter | 限流命中次数 |
| `inflight_requests` | gauge | 当前处理中的请求数 |
| `shutdown_duration_seconds` | gauge | 最近一次优雅关闭耗时 |
| `http_requests_2xx_total` | counter | 2xx 响应数 |
| `http_requests_3xx_total` | counter | 3xx 响应数 |
| `http_requests_4xx_total` | counter | 4xx 响应数 |
| `http_requests_5xx_total` | counter | 5xx 响应数 |
| `http_request_duration_ms_bucket` | histogram | 延迟分布桶（le=5/10/25/50/100/250/500/1000/2000/5000/+Inf） |
| `http_request_duration_ms_sum` | counter | 延迟累计值（毫秒） |
| `uptime_seconds` | gauge | 进程运行时长 |

指标由 `loggingMiddleware` 在请求结束时自动采集，无需手动埋点。所有请求（含被中间件拒绝的 4xx/5xx）均会计入对应状态码计数。

### 健康检查

```bash
curl localhost:8080/health   # OK
curl localhost:8080/readyz   # OK
```

---

## Cloudflare CIDR 自动同步

- 全量：每日 03:00
- 增量：每 6 小时
- 脚本：`/etc/ip-lookup/update-cloudflare-ip.sh`
- 产物：

| 组件 | 配置文件 | 加载方式 |
|------|----------|----------|
| Nginx | `/etc/nginx/conf.d/cloudflare-realip.conf` | `nginx -s reload` |
| Caddy | `/etc/caddy/cloudflare-trusted.conf` | `caddy reload` |
| Go | `/etc/ip-lookup/cf-cidrs.txt` | fsnotify 热加载（+ 5m 兜底定时器） |
| nftables | `/etc/nftables/cloudflare-cidr.nft` | `systemctl reload nftables`（仅 nftables 已安装时生成） |

失败处理：拉取失败保留旧配置；校验失败回滚；无变化不触发 reload。

---

## GeoIP 数据库更新

- 数据库：GeoLite2-City.mmdb（必需）+ GeoLite2-ASN.mmdb（可选，提供 ASN）
- 路径：`/var/lib/ip-lookup/GeoLite2-City.mmdb`、`/var/lib/ip-lookup/GeoLite2-ASN.mmdb`
- 更新：Go 进程通过 fsnotify 自动检测文件变化并热加载，无需重启
- 热重载：修改 `geoip_enabled`/`geoip_db_path`/`geoip_asn_db_path` 后配置热加载自动重建查询器，无需重启
- 地名本地化：按 `Accept-Language` 返回 `zh-CN`/`en`（仅简体中文浏览器返回中文）

### 手动更新

```bash
sudo MAXMIND_LICENSE_KEY=your_key /etc/ip-lookup/update-geoip.sh
```

### 自动更新（cron）

```bash
# /etc/cron.d/geoip-update
0 4 * * 0 root MAXMIND_LICENSE_KEY=your_key /etc/ip-lookup/update-geoip.sh
```

---

## 广告配置热更新

`config.yaml` 的广告配置修改后无需重启：

```bash
# 编辑配置
vim /etc/ip-lookup/config.yaml
# Go 进程在 1 秒内自动检测并热加载
# 前端通过主 API 响应头 (X-Ad-*) 获取最新广告文案
```

---

---

## 自监控告警

系统内置轻量自监控引擎，可在不依赖外部监控系统的情况下主动发现问题。

### 配置

```yaml
monitoring:
  enabled: false                    # 总开关，默认关闭（调试阶段保持关闭）
  check_interval: 60s               # 检查周期
  alert_cooldown: 10m               # 告警冷却时间，防止重复告警
  webhook_configs:                  # Alertmanager 兼容 webhook（支持多目标）
    - url: "https://your-domain/your-path/alertmanager"
      send_resolved: true           # 未指定时默认 true
      http_config:
        authorization:
          type: Bearer
          credentials: "YOUR_ADAPTER_TOKEN"
  error_rate_threshold: 0.05        # 5xx 错误率阈值（5%）
  p99_latency_threshold_ms: 2000    # P99 延迟阈值（2000ms）
  rate_limit_hit_rate_threshold: 0.10  # 限流命中率阈值（10%）
```

### 触发动作

阈值超限时：
1. 日志输出 `WARN` 级别告警，含指标名、当前值、阈值
2. 遍历 `webhook_configs`，异步发送 Alertmanager v4 格式 webhook 请求
3. 阈值恢复时，若 webhook 的 `send_resolved` 为 `true`（默认），发送 `resolved` 通知

### Webhook 负载格式（Alertmanager v4）

```json
{
  "version": "4",
  "groupKey": "{}:{alertname=\"error_rate\"}",
  "status": "firing",
  "receiver": "ip-lookup",
  "groupLabels": {},
  "commonLabels": {
    "alertname": "error_rate",
    "severity": "warning",
    "instance": "ip-lookup"
  },
  "commonAnnotations": {
    "summary": "Error rate 10.00% exceeds threshold 5.00%",
    "value": "0.1000",
    "threshold": "0.0500"
  },
  "externalURL": "",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "error_rate",
        "severity": "warning",
        "instance": "ip-lookup"
      },
      "annotations": {
        "summary": "Error rate 10.00% exceeds threshold 5.00%",
        "value": "0.1000",
        "threshold": "0.0500"
      },
      "startsAt": "2026-07-24T12:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "",
      "fingerprint": "a1b2c3d4e5f6a7b8"
    }
  ]
}
```

`status` 为 `firing` 时 `endsAt` 为零值；`resolved` 时 `endsAt` 为恢复时间。

### 多目标推送

`webhook_configs` 是数组，可配置多个目标。每个目标可独立设置 `send_resolved` 和 `http_config.authorization`：

```yaml
webhook_configs:
  - url: "https://team-a.example.com/alertmanager"
    send_resolved: true
  - url: "https://team-b.example.com/alertmanager"
    send_resolved: false
    http_config:
      authorization:
        type: Bearer
        credentials: "team-b-token"
```

### 环境变量覆盖

| 变量 | 对应配置 |
|------|----------|
| `MONITORING_ENABLED` | `monitoring.enabled` |

> `webhook_configs` 仅支持 YAML 配置，不提供环境变量覆盖。

---

## 故障处理

### 服务无法启动

```bash
systemctl status ip-lookup
journalctl -u ip-lookup -n 50
```

常见原因：配置文件错误、端口被占用、日志目录权限不足。

### 限流误伤

```bash
# 方式一：临时关闭限速（热加载，无需重启）
echo "RATE_ENABLED=false" >> /etc/ip-lookup/env
# 方式二：调高单 IP 限额
echo "RATE_PER_IP=30" >> /etc/ip-lookup/env
systemctl restart ip-lookup
```

### 双栈问题

确认两端口均在监听：

```bash
ss -tlnp | grep -E '808[01]'
```

### GeoIP 不生效

```bash
# 检查数据库文件是否存在
ls -la /var/lib/ip-lookup/GeoLite2-City.mmdb
# 检查配置
grep geoip /etc/ip-lookup/config.yaml
# 验证 API 返回
curl -H "Accept: application/json" localhost:8080/
```

### Webhook 告警问题

```bash
# 查看告警触发日志
journalctl -u ip-lookup -g "self-monitoring" --since "1 hour ago"

# 查看 webhook 发送结果
journalctl -u ip-lookup -g "monitor: webhook" --since "1 hour ago"

# 检查 webhook_configs 配置
grep -A 10 'webhook_configs' /etc/ip-lookup/config.yaml

# 手动测试 webhook 目标可达性（含 Bearer 认证）
curl -X POST https://your-domain/your-path/alertmanager \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"version":"4","status":"firing","receiver":"ip-lookup","groupLabels":{},"commonLabels":{},"commonAnnotations":{},"externalURL":"","alerts":[]}'
```

常见原因：
- `monitoring.enabled` 为 `false`
- `webhook_configs[].url` 不可达或证书无效
- `authorization.type` 非 `Bearer`（仅支持 Bearer）
- `check_interval` 内流量不足，delta 为 0 未触发阈值计算

---

## DDoS/CC 应急流程

1. Cloudflare Under Attack 模式
2. 临时收紧限流（降低单 IP 至 5 req/min）
3. 黑名单注入（`IP_DENYLIST` 环境变量）
4. 记录事件时间、影响范围、处理措施

---

## 备份

```bash
tar czf /var/backups/ip-lookup-$(date +%Y%m%d).tar.gz /etc/ip-lookup/ /var/lib/ip-lookup/
```
