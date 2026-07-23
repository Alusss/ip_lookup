# IP 查询工具站 - 变量占位符说明

本文件列出项目中所有 `{{VAR}}` 占位符及其默认值。

---

## 环境变量（生产部署）

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `{{ORG}}` | `my-org` | GitHub 组织/仓库名称（用于文档链接） |
| `{{DOMAIN}}` | `ip.iohow.com` | 主站域名 |
| `{{DOMAIN_IP4}}` | `ip4.iohow.com` | IPv4 API 域名（仅 A 解析） |
| `{{DOMAIN_IP6}}` | `ip6.iohow.com` | IPv6 API 域名（仅 AAAA 解析） |
| `{{LISTEN_ADDR_V4}}` | `127.0.0.1` | IPv4 监听地址 |
| `{{LISTEN_ADDR_V6}}` | `::1` | IPv6 监听地址 |
| `{{PORT_V4}}` | `8080` | IPv4 监听端口 |
| `{{PORT_V6}}` | `8081` | IPv6 监听端口 |
| `{{RATE_PER_IP}}` | `10` | 单 IP 限流（req/min） |
| `{{RATE_PER_IP_BURST}}` | `5` | 单 IP burst |
| `{{RATE_GLOBAL_BURST}}` | `1000` | 全局限流 burst |
| `{{RATE_GLOBAL_RATELIMIT}}` | `1000` | 全局限流 rate |
| `{{SHUTDOWN_TIMEOUT}}` | `15` | 优雅退出超时（秒） |
| `{{API_AD_ENABLED}}` | `true` | API 广告开关（直接访问 API 时展示） |
| `{{API_AD_TEXT_ZH}}` | — | 中文 API 广告文案 |
| `{{API_AD_URL_ZH}}` | — | 中文 API 广告跳转链接 |
| `{{API_AD_TEXT_EN}}` | — | 英文 API 广告文案 |
| `{{API_AD_URL_EN}}` | — | 英文 API 广告跳转链接 |
| `{{WEB_AD_ENABLED}}` | `true` | Web 广告开关（前端页面顶栏展示） |
| `{{WEB_AD_TEXT_ZH}}` | — | 中文 Web 广告文案 |
| `{{WEB_AD_URL_ZH}}` | — | 中文 Web 广告跳转链接 |
| `{{WEB_AD_TEXT_EN}}` | — | 英文 Web 广告文案 |
| `{{WEB_AD_URL_EN}}` | — | 英文 Web 广告跳转链接 |
| `{{LOG_LEVEL}}` | `info` | 日志级别（debug/info/warn/error） |
| `{{LOG_FILE_MAX_SIZE}}` | `50` | 日志文件最大大小（MB） |
| `{{LOG_FILE_MAX_AGE}}` | `30` | 日志文件最大保留天数 |
| `{{LOG_FILE_BACKUPS}}` | `7` | 日志文件备份数量 |
| `{{CORS_ENABLED}}` | `true` | CORS 开关 |
| `{{JSON_API_ENABLED}}` | `true` | JSON API 开关（Accept: application/json） |
| `{{LOG_IP_MASKING}}` | `true` | 日志 IP 脱敏开关 |

---

## 安全配置

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `{{MAX_HEADER_BYTES}}` | `1024` | 最大 Header 字节数 |
| `{{READ_HEADER_TIMEOUT}}` | `5` | 读取 Header 超时（秒） |
| `{{READ_TIMEOUT}}` | `10` | 读取 Body 超时（秒） |
| `{{WRITE_TIMEOUT}}` | `10` | 写入超时（秒） |
| `{{IDLE_TIMEOUT}}` | `60` | Idle 连接超时（秒） |
| `{{MAX_CONNS_PER_IP}}` | `8` | 单 IP 最大并发连接数 |
| `{{TRUSTED_PROXY_CIDRS}}` | ``（空） | 可信代理 CIDR 列表，逗号分隔 |
| `{{IP_DENYLIST}}` | ``（空） | IP 黑名单，逗号分隔，匹配则返回 403 |
| `{{UA_DENYLIST}}` | ``（空） | User-Agent 黑名单，逗号分隔子串匹配 |

---

## 可观测性配置

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `{{METRICS_LISTEN_ADDR}}` | `127.0.0.1:9090` | Prometheus 指标监听地址（仅本地可达） |
| `{{PROMETHEUS_PORT}}` | `9090` | Prometheus 指标端口（legacy，见 METRICS_LISTEN_ADDR） |

---

## 自监控告警配置

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `{{MONITORING_ENABLED}}` | `false` | 自监控总开关（默认关闭，调试阶段保持关闭） |
| `{{MONITORING_WEBHOOK_URL}}` | ``（空） | Webhook 推送地址（留空不推送） |
| `{{MONITORING_WEBHOOK_TYPE}}` | `generic` | Webhook 推送格式：`generic`（通用 JSON）或 `dingtalk`（钉钉消息） |

---

## GeoIP 配置（可选）

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `{{GEOIP_ENABLED}}` | `false` | GeoIP 地理位置查询开关 |
| `{{GEOIP_DB_PATH}}` | `/var/lib/ip-lookup/GeoLite2-City.mmdb` | GeoLite2 数据库路径 |
| `{{MAXMIND_LICENSE_KEY}}` | — | MaxMind 许可证密钥（自动更新用） |

---

## Cloudflare CIDR 自动同步配置

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `{{CF_CIDR_SYNC_CRON_FULL}}` | `0 3 * * *` | 全量同步 cron 表达式 |
| `{{CF_CIDR_SYNC_CRON_INCR}}` | `0 */6 * * *` | 增量校验 cron 表达式 |

---

## 前端配置

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `{{CWA_ID}}` | `{{CWA_PLACEHOLDER}}` | Cloudflare Web Analytics ID |

---

## 变更历史

- 2026-07-22: 初始版本（v0.1）
- 2026-07-22: 拆分 API 广告与 Web 广告配置，增加 GeoIP/JSON API 配置
- 2026-07-23: 增加自监控告警配置（MONITORING_ENABLED / MONITORING_WEBHOOK_URL / MONITORING_WEBHOOK_TYPE）
