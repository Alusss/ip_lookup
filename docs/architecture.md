# IP 查询工具站 - 架构设计

## 1. 系统架构

```mermaid
flowchart LR
    U["用户浏览器<br/>Chrome/Safari/Firefox/iOS"] --> CF["Cloudflare<br/>CDN+WAF+RateLimit"]
    CF -->|ip.iohow.com| PAGES["Cloudflare Pages<br/>静态前端"]
    CF -->|ip4.iohow.com A| ORIG4["源站<br/>Caddy/Nginx（TLS 终止）"]
    CF -->|ip6.iohow.com AAAA| ORIG6["源站<br/>Caddy/Nginx（TLS 终止）"]
    ORIG4 --> GOv4["Go Backend<br/>127.0.0.1:8080"]
    ORIG6 --> GOv6["Go Backend<br/>[::1]:8081"]
    GOv4 -->|slog + lumberjack| LOG["日志轮转<br/>/var/log/ip-lookup/"]
    GOv4 -->|/metrics| MON["Prometheus 指标<br/>(含状态码/延迟分布)"]
    GOv4 -.->|自监控引擎<br/>阈值检查+Webhook| ALERT["告警通知<br/>Alertmanager v4"]
    GOv4 -->|geoip2-golang| GEO["GeoLite2 City<br/>.mmdb 文件<br/>fsnotify 热加载"]
    PAGES -.fetch ip4.iohow.com.-> ORIG4
    PAGES -.fetch ip6.iohow.com.-> ORIG6
```

## 2. DNS 拓扑

| 域名 | 记录类型 | 解析目标 | 说明 |
|------|----------|----------|------|
| `ip.iohow.com` | A + AAAA | Cloudflare CDN (Proxied) | 主站，双栈，静态前端 |
| `ip4.iohow.com` | A（仅） | 源站 IPv4 | IPv4 API，经 CDN 代理 |
| `ip6.iohow.com` | AAAA（仅） | 源站 IPv6 | IPv6 API，经 CDN 代理 |

## 3. 数据流

### 3.1 前端页面加载

```
浏览器 → ip.iohow.com → Cloudflare CDN → Cloudflare Pages (静态文件)
                                            ↓
                                       HTML 含内联 CSS + defer JS
                                             ↓
                               并行请求：
                               ├── fetch ip4.iohow.com/ (X-Client: web) → IPv4 地址 + 广告配置(响应头)
                               └── fetch ip6.iohow.com/ (X-Client: web) → IPv6 地址
                                             ↓
                               渲染 IPv4 + IPv6 + 广告栏（可选）
```

### 3.2 API 请求

```
浏览器/curl → ip4/ip6.iohow.com → Cloudflare CDN (WAF + RateLimit)
                                    → Caddy/Nginx (TLS 终止 + 真实 IP + 前置限流)
                                    → Go Backend
                                      ├─ Accept: application/json → JSON (含 GeoIP)
                                      ├─ X-Client: web → 纯 IP
                                      └─ 直接访问 → 广告 + IP
```

## 4. 四层防御矩阵

| 层级 | 组件 | 职责 |
|------|------|------|
| L1 CDN + Edge | Cloudflare + _headers | WAF、DDoS 防护、边缘限流、TLS、CSP 安全策略 |
| L2 Web 服务器 | Caddy/Nginx | 真实 IP 还原、前置限流、安全 Header、TLS 终止 |
| L3 Go 应用 | ip-lookup | IP 解析、应用层限流、广告双轨、JSON API、GeoIP 查询、结构化日志、指标、自监控告警 |
| L3 中间件链 | 11 层顺序 | requestID → realIP → recovery → securityHeaders → metrics(连接数) → methodCheck → bodyRejection → denylist → connLimit → cors → logging(请求计数+延迟+状态码) |
| L4 系统 | nftables + systemd | 源站防火墙（仅 CF CIDR）、资源隔离 |

## 5. 关键决策记录

- `docs/adr/0001-choose-go-native-http.md`：使用 Go 标准库 net/http，不引入框架
- `docs/adr/0002-choose-caddy.md`：Caddy 为主（自动 HTTPS），Nginx 备选
- `docs/adr/0003-frontend-no-framework.md`：前端原生，不引入框架

## 6. 安全架构

### 6.1 IP 信任链

```
RemoteAddr → 检查来源 IP ∈ CF CIDR？
  ├─ 是 → 取 CF-Connecting-IP → 校验合法性 → 用于限流/日志
  └─ 否 → 检查来源 IP ∈ TRUSTED_PROXY_CIDRS？
       ├─ 是 → 取 X-Forwarded-For 最右可信跳 → 校验
       └─ 否 → 取 RemoteAddr → 拒绝信任任何代理头
```

### 6.2 限流策略

- 单 IP Token Bucket：10 req/min, burst 5（sync.Map 实现，无锁争用）
- 全局 Token Bucket：5000 req/s, burst 5000
- 超出 → 429 + Retry-After
- 5 分钟 TTL 清理空闲桶

### 6.3 性能优化

- **GeoIP 缓存**：10K 条目 LRU 缓存，重复 IP 查询命中率 ~80%，省去 mmdb 二分查找 (~200μs)
- **Context 传递真实 IP**：`realIPMiddleware` 前置提取一次，通过 Context 传递至各中间件，消除 4 次重复 CIDR 扫描
- **连接数计数分片**：16 分片消除全局 Mutex 争用，支撑 10K+ 并发连接
- **Per-IP 限流器 sync.Map**：`LoadOrStore` 原子语义替代 RWMutex 双重检查锁定

## 7. 可观测性

| 端点/组件 | 说明 |
|-----------|------|
| `/` | IP 查询（纯文本或 JSON，按 Accept 协商） |
| `/all` | 完整归属信息 JSON（`all_api_enabled` 开关控制；关时等同 `/`） |
| `/ad-config` | 前端广告配置 |
| `/health` | 存活检查 |
| `/readyz` | 就绪检查 |
| `/metrics` | Prometheus 指标（请求数/限流/在线连接/关闭耗时 + 按状态码计数 + 延迟分布直方图 + 运行时间） |
| 自监控引擎 | 内置周期检查：错误率、P99 延迟、限流命中率 → 超阈值时日志 WARN + Webhook 推送（Alertmanager v4） |

## 8. 部署架构

- **默认**：systemd 管理 Go 二进制（双栈监听 :8080/:8081），Caddy/Nginx 做反向代理
- **可选**：Docker 多阶段构建 + distroless 镜像
- **CF CIDR 同步**：每日全量 + 每6h增量，Nginx/Caddy/Go/nftables 四份配置（nftables 可选）
- **GeoIP**（可选）：GeoLite2 City + ASN 免费数据库，fsnotify 热加载 + 配置热重载，cron 每周更新
- **源站防火墙**：nftables 仅放行 CF CIDR
