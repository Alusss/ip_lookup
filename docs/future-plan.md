# Future Plan

## P2.9 水平扩缩容方案

当前状态：单进程设计，不支持水平扩展。

规划方案：
- 多实例部署时，全局速率限制（`GlobalRateLimiter`）为进程级，多实例间无共享状态，每个实例独立限流
- 若需严格全局限流，可引入 Redis 作为中心化令牌桶（`github.com/go-redis/redis_rate`），或接受近似值（各实例独立限流）
- 前端负载均衡：Cloudflare DNS 轮询 / L4 LB（如 HAProxy）分发流量到多台后端实例
- 无 session affinity 需求（服务无状态），水平扩展天然友好
- 监控指标聚合：各实例独立暴露 `/metrics`，由 Prometheus 统一拉取

## P2.10 健康检查扩展

当前状态：`/health` 和 `/readyz` 仅返回固定 OK。

规划方案：
- `/health`（存活检查）：当前实现足够——进程存活即返回 200
- `/readyz`（就绪检查）：可扩展为检查：
  1. GeoIP 数据库是否已加载（若 `geoip_enabled=true`）
  2. 配置热加载 watcher 是否正常运作
  3. CF CIDR 列表是否非空（若配置了路径）
- 扩展后返回 JSON 格式：`{"status":"ok","checks":{"geoip":"ok","config_watcher":"ok"}}`
- 避免在健康检查路径中引入外部依赖调用，保持轻量
