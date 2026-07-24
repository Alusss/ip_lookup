# IP 查询工具站 — 系统技术设计与运维手册

**版本**: 1.0.0 | **最后更新**: 2026-07-23 | **状态**: 生产就绪

---

## 第一部分：项目概述

### 1. 项目背景

#### 为什么开发这个项目

互联网上有大量 IP 查询工具，但大多数存在以下问题：页面臃肿、加载慢、广告泛滥、跟踪用户行为（Cookie/PII）、不提供或隐藏 JSON API、IPv6 检测需要手动点击。本项目旨在提供一款**极简、零广告（在端上）、毫秒级、SEO 友好、尊重隐私**的公网 IP 查询与 IPv6 连通性检测工具。

#### 解决什么业务问题

- **用户痛点**：需要一个快速（<200ms）、精准、无追踪的 IP 查看方式
- **开发者痛点**：需要简洁的纯文本/JSON API 以便脚本/应用集成
- **IPv6 推广**：用户需要方便地确认自己是否已启用 IPv6 以及地址信息
- **流量变现需求**：通过非侵入式的广告配置实现可持续运营，但不影响核心体验

#### 原有方案有什么不足

- 主流竞品（ipify, ipinfo, icanhazip 等）功能单一或 API 有调用限制
- 中文生态的 IP 工具站普遍存在：大量广告、Cookie 追踪、JS 探测、移动端体验差
- 无多语言支持或仅英文
- 开源方案多为单体后端+一体化 UI，不利于灵活部署

#### 新系统产生的价值

1. **用户价值**：纯 IP 查询 <10ms，零 Cookie 零追踪，IPv4/IPv6 自动并行检测
2. **开发者价值**：提供标准纯文本和 JSON API（携带 `Accept: application/json` 即可），开源可自建
3. **运营价值**：无数据库无状态，单二进制部署，运维成本趋近于零；内置广告配置热更新，无需重启
4. **SEO 价值**：通过工具页+知识页（IPv6 科普）双轨支撑长尾搜索流量

### 2. 项目目标

| 维度 | 目标 |
|------|------|
| **业务目标** | 日活 ~10K UV，API 调用量 ~50K req/day |
| **技术目标** | P99 延迟 <50ms，无数据库无状态，单实例支撑 ≥1000 req/s，systemd-analyze 安全评分 ≤ 3.0 |
| **用户目标** | 首屏反馈 <1.2s，IPv4+IPv6 自动并行检测，零点击获取结果，隐私零收集 |
| **运营目标** | 运维人员 <0.1 FTE，月服务器成本 <$10，广告配置热更新无需重启 |

### 3. 项目定位

| 维度 | 说明 |
|------|------|
| **系统服务对象** | 公网互联网用户 + 开发者 + SEO 长尾流量 |
| **使用场景** | 查询公网 IP、检测 IPv6 连通性、开发者 API 集成、网络排障 |
| **核心价值** | 极速、零隐私收集、双栈自动检测、开源可自建 |
| **产品定位** | 极简、零广告（端上可配置）、毫秒级、SEO 友好的公网 IP 查询与 IPv6 连通性检测工具 |

### 4. 项目范围

| 包含 | 不包含 | 未来规划 |
|------|--------|----------|
| IPv4/IPv6 地址查询 | 用户系统/注册/登录 | 子网计算工具页 |
| IPv6 连通性检测 | 数据库存储 | 批量 IP 查询 API |
| 纯文本 API | 消息中间件 | 多语种扩展 |
| JSON API（含可选 GeoIP） | 实时日志流 | 源站多 region 支持 |
| 前端广告展示（可关闭/可配置） | IP 地理位置历史追踪 | 企业级付费 API |
| 开发者 API（curl/脚本） | 第三方集成 SDK | PWA Service Worker |
| SEO 内容矩阵（知识页） | | |
| 内置监控告警 | | |

---

## 第二部分：需求分析

### 1. 用户角色分析

| 用户类型 | 使用方式 | 核心需求 | 权限 |
|----------|----------|----------|------|
| **普通用户** | 浏览器访问首页 | 快速查看自己的公网 IP 和 IPv6 状态 | 无需认证，仅 GET |
| **开发者** | curl/编程语言 HTTP 调用 | 纯文本或 JSON 格式获取 IP、GeoIP 数据 | 无需认证，仅 GET |
| **搜索引擎** | 爬虫索引工具页和知识页 | 结构化数据、语义化 HTML | 无限制 |
| **运维人员** | systemd + Caddy/Nginx | 部署、配置、监控、CF CIDR 同步、GeoIP 更新 | 系统 root/sudo |

**用户流程（普通用户）**：
```
1. 浏览器输入 ip.iohow.com → Cloudflare Pages 返回静态 HTML
2. HTML 加载完毕，JS 自动并行发起两个请求：
   ├─ fetch ip4.iohow.com/ (X-Client: web) → 获取 IPv4 地址
   └─ fetch ip6.iohow.com/ (X-Client: web) → 获取 IPv6 地址
3. 结果自动渲染到页面：IPv4 展示、IPv6 展示（或"未启用"提示）
4. 用户可点击复制、刷新（60s 节流）
```

**用户流程（开发者）**：
```
curl ip4.iohow.com/   → "203.0.113.42" (纯文本)
curl -H "Accept: application/json" ip4.iohow.com/   → {"ip":"...","version":"IPv4","city":"...","country":"...","isp":"...","asn":"..."}
curl ip4.iohow.com/all   → 同上 JSON（需 all_api_enabled=true；关闭时等同 /；api_ad_enabled=true 时附带 ad 字段）
```

### 2. 功能需求

#### 模块 A：后端 IP 查询核心

| 项 | 说明 |
|------|--------|
| **功能** | 接收 HTTP GET 请求，识别真实客户端 IP，返回纯文本或 JSON 格式 |
| **用途** | 用户查询自己公网 IP、开发者 API 集成 |
| **业务流程** | 请求到达 → IP 提取（信任链）→ 限流检查 → 广告判断 → 响应渲染 → 日志记录 |
| **输入** | HTTP GET 请求，可选 `Accept: application/json`、`X-Client: web` 头 |
| **处理** | 通过 `IPExtractor` 从 `RemoteAddr` + 信任链计算真实 IP → `rate.Limiter` 限流 → `handler.go` 内容协商 |
| **输出** | `text/plain` 纯 IP 或 `application/json` JSON 对象 |

#### 模块 B：前端展示与交互

| 项 | 说明 |
|------|--------|
| **功能** | 加载时自动获取并展示 IPv4/IPv6 地址，支持复制、刷新 |
| **用途** | 普通用户零操作获得查询结果 |
| **业务流程** | DOMContentLoaded → 并行 fetch ip4/ip6 → 根据结果更新 UI 状态（5 态状态机） |
| **输入** | 浏览器原生 fetch API，携带 `X-Client: web` 头 |
| **处理** | fetch → 解析响应文本/头 → 更新 DOM |
| **输出** | 页面更新 IP 地址展示、状态消息、广告栏 |

#### 模块 C：广告配置

| 项 | 说明 |
|------|--------|
| **功能** | Web 广告通过 `/` 响应头 `X-Ad-*` 下发；另有 `/ad-config` 独立端点可选；直接 API 访问也可配置广告文案 |
| **用途** | 非侵入式变现，兼顾 Web 和 API 两种场景 |
| **业务流程** | 后端 YAML/ENV 配置广告文案 → fsnotify 热加载 → `/` 响应注入 `X-Ad-*` 头 → 前端读取渲染/隐藏 |
| **输入** | 前端 `GET /`（携带 `X-Client: web`）或 `GET /ad-config`，携带 `Accept-Language` |
| **处理** | `Config.GetWebAdConfig` 按语言返回对应文案 |
| **输出** | `/` 响应头 `X-Ad-Enabled/Text/URL`；`/ad-config` 返回 JSON `{"web":{"enabled":true,"text":"...","url":"..."}}` |

#### 模块 D：GeoIP 地理位置查询

| 项 | 说明 |
|------|--------|
| **功能** | 可选模块，根据 IP 查询城市/国家/ISP/ASN |
| **用途** | 为 JSON API 用户提供 IP 归属地信息 |
| **业务流程** | JSON API 请求 -> `geoip.go:Lookup(ip, lang)` -> City/ASN 库查找 -> LRU 缓存 -> 响应 |
| **输入** | JSON API 请求（`Accept: application/json`）或 `/all` 路由 |
| **处理** | GeoLite2 City + ASN 库解析 -> 按 `Accept-Language` 选 `zh-CN`/`en` 地名（回退 en）-> ASN 编号+组织名 |
| **输出** | 附加在 JSON 响应中的 `city`/`country`/`isp`/`asn` 字段 |

#### 模块 E：可观测性

| 项 | 说明 |
|------|--------|
| **功能** | 健康检查、就绪检查、Prometheus 指标、自监控告警 |
| **用途** | 运维监控、异常发现、容量规划 |
| **输入** | `/health`、`/readyz`、`/metrics` 端点请求 |
| **处理** | 内置监控引擎周期性计算错误率/P99/限流率 → 阈值超过时日志 + Alertmanager v4 Webhook 推送 |
| **输出** | Prometheus 文本格式指标、Alertmanager v4 JSON 告警（含 firing/resolved 状态） |

#### 模块 F：Cloudflare CIDR 自动同步

| 项 | 说明 |
|------|--------|
| **功能** | 自动拉取 Cloudflare 出口 IP 段，更新 Go/Caddy/Nginx/nftables 配置 |
| **用途** | 维持真实 IP 信任链与源站防火墙的准确性，因 CF IP 段会变化 |
| **业务流程** | cron/systemd timer -> 拉取官方列表 -> 校验 -> 原子替换 -> reload/热加载 |
| **输入** | Cloudflare 官方 IP 列表 |
| **处理** | shell 脚本解析 -> 同时生成四份配置 -> 校验长度/格式 -> 原子 mv |
| **输出** | 更新后的 `cf-cidrs.txt`、`cloudflare-realip.conf`、`cloudflare-trusted.conf`、`cloudflare-cidr.nft`（nftables 仅在已安装时生成） |

### 3. 非功能需求

| 类别 | 要求 | 实现方式 |
|------|------|----------|
| **性能** | P99 <50ms 内部处理；LCP <1.2s | 无状态内存处理、GeoIP LRU 缓存（10K 条目）、纯静态前端、内联 CSS |
| **安全** | 四层防御（CDN→Web→App→OS）、HSTS、CSP、systemd 加固 | 11 层中间件链、nftables、systemd-analyze ≤ 3.0 |
| **可靠性** | 无单点故障，优雅退出 <15s | 双栈独立监听、SIGTERM 上下文取消、panic recovery |
| **扩展性** | 水平扩展：加机器改 DNS 即可 | 完全无状态、无数据库、仅依赖 mmdb 文件（热加载） |
| **兼容性** | Chrome/Firefox/Safari/iOS/Android 近 2 年版本；标准 HTTP 客户端 | 标准 HTTP 协议、CORS 开放、纯文本/JSON 双模式 |
| **隐私合规** | 不收集 PII、不使用 Cookie、日志 IP 脱敏 | `log_ip_masking: true`、`_headers` CSP 无外部请求、无 GA |

---

## 第三部分：系统总体设计

### 1. 系统整体架构

```
用户浏览器 / curl
     │
     ▼
┌──────────────────────────────────────────────────────┐
│              Cloudflare CDN (WAF + DDoS)              │
│   ip.iohow.com → Cloudflare Pages (静态前端)          │
│   ip4.iohow.com → 源站 (TLS + 真实 IP 还原)           │
│   ip6.iohow.com → 源站 (TLS + 真实 IP 还原)           │
└──────────────────┬───────────────────────────────────┘
                   │ HTTPS
                   ▼
┌──────────────────────────────────────────────────────┐
│              Caddy / Nginx (可选，二选一)              │
│   - TLS 终止                                          │
│   - Cloudflare 真实 IP 还原 (trusted_proxies)         │
│   - 前置限流 (limit_req)                              │
│   - 反向代理到 Go backend                             │
│   ip4: bind 0.0.0.0 → 127.0.0.1:8080                  │
│   ip6: bind [::] → [::1]:8081                         │
└──────────────────┬───────────────────────────────────┘
                   │ HTTP
                   ▼
┌──────────────────────────────────────────────────────┐
│               Go Backend (ip-lookup)                   │
│   双 http.Server 实例：                                │
│   ┌─ IPv4 :8080 (ListenAndServe)                     │
│   └─ IPv6 :8081 (ListenAndServe)                     │
│   共享 Handler + 11 层中间件链                         │
│   ┌──────────────────────────────────────────────┐   │
│   │ 中间件链：                                    │   │
│   │ requestID → realIP → recovery →              │   │
│   │ securityHeaders → metricsInflight →          │   │
│   │ methodCheck → bodyRejection → denylist →     │   │
│   │ connLimit → cors → logging                   │   │
│   └──────────────────────────────────────────────┘   │
│                                                       │
│   内置组件：                                           │
│   ├─ Config (YAML + ENV + fsnotify 热加载)            │
│   ├─ IPExtractor (CF CIDR + fsnotify 热加载)          │
│   ├─ PerIPRateLimiter (sync.Map + cleanup goroutine)  │
│   ├─ GlobalRateLimiter                                │
│   ├─ GeoIP (geolite2 + LRU 缓存 + fsnotify 热加载)    │
│   ├─ Metrics (atomic + prometheus 格式)               │
│   └─ Monitor (自监控引擎 + Alertmanager v4 webhook)     │
│                                                       │
│   独立端口 :20013 → /metrics                           │
└──────────────────────────────────────────────────────┘
     │           │           │
     ▼           ▼           ▼
┌─────────┐ ┌─────────┐ ┌─────────┐
│ 日志    │ │ 指标    │ │ GeoIP   │
│ journald│ │Prometheus│ │.mmdb    │
│ +lumber │ │ 格式    │ │ 文件    │
│ jack    │ │4xx/5xx  │ │fsnotify │
└─────────┘ └─────────┘ └─────────┘
```

### 2. 技术架构

| 层 | 技术选型 | 选择理由 |
|----|----------|----------|
| **前端** | 原生 HTML + CSS + JS（ES2020+） | 零构建依赖、零运行时框架、首屏极速、Cloudflare Pages 原生支持 |
| **后端** | Go 1.23 + `net/http` | 单二进制 (~5MB)、零运行时、内存安全、并发原生、无 GC 抖动 |
| **Web 服务器** | Caddy（主）+ Nginx（备） | Caddy 自动 HTTPS（Cloudflare DNS-01）、配置极简；Nginx 为传统运维人员备选 |
| **CDN** | Cloudflare | 全球边缘加速、WAF、DDoS 防护、免费档已够用 |
| **前端托管** | Cloudflare Pages | 零服务器、全球 CDN、自动 HTTPS、_headers/_redirects 原生支持 |
| **配置管理** | YAML + 环境变量双层覆盖 + fsnotify 热加载 | 兼顾可读性和部署灵活性，热加载消除重启 |
| **限流** | `golang.org/x/time/rate` | Go 官准 token bucket 实现，无外部依赖 |
| **GeoIP** | `geoip2-golang` + MaxMind GeoLite2 | 免费数据库、离线查询、无外部 API 调用延迟 |
| **日志** | `log/slog` JSON + `lumberjack` | 标准库结构化日志 + 轮转 |
| **指标** | Prometheus 文本格式（内置 /metrics） | 零外部依赖暴露标准指标 |
| **部署** | systemd（主）+ Docker（可选） | 单二进制 <5MB → systemd 是最优解；Docker 用于 CI artifact 和回滚兜底 |

### 3. 系统模块划分

| 模块 | 文件 | 职责 | 依赖 |
|------|------|------|------|
| **配置管理** | `config.go` | 加载 YAML、环境变量覆盖、验证、fsnotify 热加载 | `fsnotify`、`yaml.v3` |
| **IP 提取** | `ip_extract.go` | 真实 IP 提取（信任链）、CF CIDR 热加载 | `fsnotify` |
| **限流** | `ratelimit.go` | 单 IP Token Bucket + 全局 Token Bucket | `golang.org/x/time/rate` |
| **广告** | `ad.go` | 判断是否为 Web 客户端 | 无 |
| **处理器** | `handler.go` | HTTP 路由分发、内容协商（纯文本/JSON）、广告注入 | 其他所有模块 |
| **中间件** | `middleware.go` | 11 层 HTTP 中间件链 | `handler.go`、Config、Metrics、IPExtractor |
| **指标** | `metrics.go` | Prometheus 格式指标收集 | 无 |
| **GeoIP** | `geoip.go` | IP 地理查询、LRU 缓存、DB 热加载 | `geoip2-golang` |
| **错误集中管理** | `errors.go` | 错误消息常量 + 状态码映射 | 无 |
| **监控** | `monitor.go` | 内置自监控引擎、阈值检查、Alertmanager v4 Webhook 推送（多目标 + Bearer 认证 + resolved 恢复通知） | Config、Metrics |
| **缓存** | `cache.go` | 泛型 LRU 缓存 | 无 |
| **入口** | `main.go` | 初始化所有组件、启动双栈 HTTP Server、优雅退出 | 所有模块 |

### 4. 核心运行流程

```
用户 curl ip4.iohow.com/
    │
    ▼
Cloudflare CDN → 检查 WAF/安全规则
    │
    ▼
Caddy → TLS 终止 → 还原真实 IP (CF-Connecting-IP)
    │
    ▼
Go Backend :8080
    │
    ├─ 中间件链开始
    │   ├─ requestIDMiddleware → 生成 X-Request-ID
    │   ├─ realIPMiddleware → 从 extractor 提取真实 IP 存入 Context
    │   ├─ recoveryMiddleware → defer recover()
    │   ├─ securityHeadersMiddleware → X-Content-Type-Options, X-Frame-Options
    │   ├─ metricsMiddleware → incInflight / decInflight
    │   ├─ methodCheckMiddleware → 仅允许 GET
    │   ├─ bodyRejectionMiddleware → 拒绝带 body 请求
    │   ├─ denylistMiddleware → IP/UA 黑名单匹配
    │   ├─ connLimitMiddleware → 单 IP 并发 > 8 拒绝
    │   ├─ corsMiddleware → CORS 头
    │   └─ loggingMiddleware → 计时 + 日志 + 指标计数
    │
    ├─ ServeMux 路由
    │   ├─ /health → 200 OK
    │   ├─ /readyz → 200 OK (ready atomic 控制)
    │   ├─ /ad-config → JSON 广告配置
    │   ├─ /all → allHandler（开关 off 时等同 /）
    │   └─ / → rootHandler
    │
    ├─ rootHandler / allHandler 逻辑
    │   ├─ ready.Load()? → 否：503
    │   ├─ cf_only 且来源非 CF/受信代理? → 否：403
    │   ├─ rate_enabled? → 按 rate_mode 校验 global/per_ip 限流 → 否：429
    │   ├─ /all 开启? → JSON 响应 (含 GeoIP+ASN)
    │   ├─ Accept: application/json? → JSON 响应 (含 GeoIP+ASN)
    │   ├─ X-Client: web? → 纯 IP + 响应头广告
    │   └─ 直接访问 → 广告文案 + IP
    │
    └─ 返回 HTTP Response
```

---

## 第四部分：详细设计

### 模块：配置管理 (config.go)

#### 设计目标

单数据源配置，YAML + 环境变量双层覆盖，运行时热加载不重启。

#### 核心数据结构

```go
type Config struct {
    ConfigValues
    configPath string
    mu         sync.RWMutex
    watcher    *fsnotify.Watcher
    onReload   func(*Config)
}
```

ConfigValues 与 Config 分离嵌入——保证 `go vet` 零警告（避免值接收器中的互斥锁）。

#### 配置加载优先级

1. `DefaultConfig()` 填充所有默认值
2. `yaml.Unmarshal(file, cfg)` 解析磁盘 YAML 文件
3. `overrideFromEnv(cfg)` 环境变量覆盖（40+ 个环境变量对应项）
4. `cfg.validate()` 校验（广告 URL scheme 必须为 http/https）

#### 热加载机制

```go
func (cfg *Config) StartHotReload(onReload func(*Config)) error
```

1. fsnotify 监听 config.yaml 的 Write/Rename 事件
2. 200ms debounce 防止频繁写入时多次触发
3. 重新读取 → 解析 → 校验 → 原子替换 `ConfigValues`
4. 更新失败（解析错误/校验失败）保留当前配置不覆盖
5. 成功后回调 `onReload`（当前用于更新 IPExtractor 的 proxy CIDR）

### 模块：IP 提取 (ip_extract.go)

#### 设计目标

通过最小信任链原则提取真实客户端 IP，防止伪造。

#### 核心逻辑

```go
func (e *IPExtractor) RealIP(r *http.Request) (net.IP, error)
```

1. 从 `r.RemoteAddr` 提取来源 IP
2. 检查来源 IP 是否属于 Cloudflare CIDR 列表（`e.isCfIP`）
   - 是 → 取 `CF-Connecting-IP` header → 校验 → 返回
   - 备选 → 取 `X-Forwarded-For` 最右跳 → 返回
3. 检查来源 IP 是否属于自定义可信代理 CIDR（`e.isProxyIP`）
   - 是 → 取 `X-Forwarded-For` 最右跳 → 返回
4. 都不是 → 返回 RemoteAddr（直连请求）

#### Cloudflare CIDR 热加载

- fsnotify 监听 `/etc/ip-lookup/cf-cidrs.txt` 文件变更
- 变更后 200ms debounce → 原子替换 `cfCIDRs` 切片
- 兜底定时器（`cf_cidr_reload_interval`，默认 5m）周期重载，防 fsnotify 事件丢失
- 不停止现有连接

### 模块：限流 (ratelimit.go)

#### 设计目标

两档独立限流（单 IP + 全局），Token Bucket 算法，防止资源耗尽。

```go
type PerIPRateLimiter struct {
    limiters sync.Map       // key: IP string, value: *rate.Limiter
    rate     rate.Limit     // 速率 (tokens/s)
    burst    int            // 突发容量
    ttl      time.Duration  // 空闲 IP 清理周期
    stopCh   chan struct{}
}
```

| 维度 | 速率 | Burst | 清理周期 |
|------|------|-------|----------|
| 单 IP | 10 req/min (≈0.166 token/s) | 5 | 5分钟 |
| 全局 | 5000 req/s | 5000 | — |

**关键设计**：
- `sync.Map.LoadOrStore` 原子语义避免竞态
- 清理 goroutine 每 5 分钟遍历所有 limiters，删除 token 满的（即该 IP 已空闲）
- 超出返回 `429 Too Many Requests` + `Retry-After` 头
- `rate_enabled` 总开关（可临时关闭便于调试）；`rate_mode` 选择 `both`/`per_ip`/`global`；均支持热加载

### 模块：处理器 (handler.go)

#### 响应逻辑矩阵

| 请求特征 | 返回格式 | 内容 |
|----------|----------|------|
| `Accept: application/json` + `JsonApiEnabled=true` | JSON | `{"ip":"...","version":"IPv4","city":"...","country":"...","isp":"...","asn":"..."}`；`ApiAdEnabled=true` 时附带 `ad` 字段 |
| `GET /all` + `AllApiEnabled=true` | JSON | 同上（始终 JSON，含 GeoIP+ASN）；`ApiAdEnabled=true` 时附带 `ad` 字段 |
| `X-Client: web` | text/plain | 纯 IP（响应头带 X-Ad-* 广告信息） |
| `ApiAdEnabled=true` 且无 web 头 | text/plain | 两行：`广告文案 (URL)\n纯IP` |
| `ApiAdEnabled=false` 且无 web 头 | text/plain | 纯 IP |

### 模块：中间件链 (middleware.go)

#### 11 层中间件设计

| 次序 | 中间件 | 作用 | 状态码 |
|------|--------|------|--------|
| 1 | `requestIDMiddleware` | 生成 X-Request-ID 追踪 | 不拦截 |
| 2 | `realIPMiddleware` | 前置提取 IP 存入 Context | 不拦截（提取失败仅跳过） |
| 3 | `recoveryMiddleware` | defer recover() 防止 panic 崩溃 | 500 |
| 4 | `securityHeadersMiddleware` | X-Content-Type-Options, X-Frame-Options, Referrer-Policy | 不拦截 |
| 5 | `metricsMiddleware` | incInflight / decInflight | 不拦截 |
| 6 | `methodCheckMiddleware` | 仅允许 GET，URL 长度 < 256 | 405/414 |
| 7 | `bodyRejectionMiddleware` | 拒绝 ContentLength > 0 的请求 | 400 |
| 8 | `denylistMiddleware` | IP 精确/IP CIDR/UA 子串黑名单 | 403 |
| 9 | `connLimitMiddleware` | 单 IP 并发连接数限制（分片计数） | 429 |
| 10 | `corsMiddleware` | CORS 头 + OPTIONS 预检 | 204 (OPTIONS) |
| 11 | `loggingMiddleware` | 计时、结构化日志、指标计数 | 不拦截 |

**设计思想**：
- realIPMiddleware 在链最前面提取 IP，后续所有中间件通过 `r.Context().Value(realIPKey)` 获取，避免每个中间件独立调用 CIDR 校验（消除 4 次重复 CIDR 扫描）
- connLimitMiddleware 使用 16 分片的连接计数器消除全局 Mutex 争用
- loggingMiddleware 通过自定义 `responseWriter` 包裹原始 ResponseWriter 捕获真实状态码
- middleware 包装顺序 = 外层先执行：`outer(inner(handler))`，所以 early middleware 在链外层

### 模块：指标 (metrics.go)

#### 指标端点

默认 `127.0.0.1:20013/metrics`（仅本地可达），暴露 Prometheus 文本格式：

```
# HELP http_requests_total
# TYPE http_requests_total counter
http_requests_total <total>

# HELP rate_limit_hits_total
rate_limit_hits_total <count>

# HELP inflight_requests
# TYPE inflight_requests gauge
inflight_requests <current>

# HELP shutdown_duration_seconds
shutdown_duration_seconds <seconds>

# HELP http_requests_{2xx,3xx,4xx,5xx}_total
http_requests_4xx_total <count>

# HELP http_request_duration_ms_bucket{le="5|10|25|50|100|250|500|1000|2000|5000|+Inf"}
http_request_duration_ms_bucket{le="50"} <count>

# HELP http_request_duration_ms_sum
http_request_duration_ms_sum <total_ms>

# HELP uptime_seconds
uptime_seconds <seconds>
```

所有计数器使用 `atomic.Int64`，无锁无争用。

### 模块：GeoIP (geoip.go)

#### 设计目标

可选模块，通过 MaxMind GeoLite2 免费数据库查询 IP 地理位置，10K LRU 缓存消除重复查询。

```go
func (g *GeoIP) Lookup(ipStr string) *GeoLocation
```

1. `cache.Get(ipStr)` → 命中直接返回
2. `cityReader.City(ip)` → 提取城市、国家
3. `ispReader.ISP(ip)` → 提取 ISP（如 ISP 数据库存在）
4. `cache.Set(ipStr, location)` → 存入 LRU 缓存
5. 返回 `GeoLocation{City, Country, ISP}`

**热加载**：fsnotify 监听 `.mmdb` 文件变更 → 500ms debounce → 重新 Open → 原子替换 reader → Flush cache

### 模块：自监控 (monitor.go)

#### 设计目标

不依赖外部监控系统，内置轻量健康检查引擎，主动发现并推送告警。

```go
type Monitor struct {
    cfg     *Config
    metrics *Metrics
    alerted *alertState    // cooldown + firing/resolved 状态跟踪
    stopCh  chan struct{}
}
```

**检查周期**：默认 60s

**阈值**：
- 错误率 > 5%（delta 5xx / delta 总请求）
- P99 延迟 > 2000ms
- 限流命中率 > 10%

**触发动作**：
1. cooldown 检查（默认 10 分钟内不重复告警同一指标）
2. 日志 WARN 级别输出
3. 异步 Webhook 推送（Alertmanager v4 格式，支持多目标 + Bearer 认证）
4. 阈值恢复时发送 `resolved` 通知（`send_resolved` 控制，默认开启）

**配置结构**（`config.yaml` 的 `monitoring` 段）：

```yaml
monitoring:
  enabled: true
  webhook_configs:                    # 数组，支持多目标
    - url: "https://adapter.example.com/alertmanager"
      send_resolved: true             # 未指定时默认 true
      http_config:
        authorization:
          type: Bearer                # 仅支持 Bearer
          credentials: "TOKEN"
```

> 完整配置示例与 Alertmanager v4 payload 格式详见 [运维手册 - 自监控告警](operation.md#自监控告警)。

**P99 计算**：通过 11 个延迟桶的差值计算百分位，逼近真实 P99。

---

## 第五部分：数据设计

本项目**无数据库**。系统完全无状态：

| 数据类别 | 存储方式 | 生命周期 |
|----------|----------|----------|
| GeoIP 数据库 | `.mmdb` 文件（磁盘） | 外部更新，Go 进程自动热加载 |
| CF CIDR 列表 | `/etc/ip-lookup/cf-cidrs.txt`（磁盘） | 每日同步脚本更新，Go 热加载 |
| 配置 | `/etc/ip-lookup/config.yaml`（磁盘） | 手动编辑，fsnotify 热加载 |
| 日志 | `/var/log/ip-lookup/ip-lookup.log`（磁盘轮转） | lumberjack 自动轮转 |
| 运行时状态 | 内存（atomic/sync.Map） | 进程重启丢失 |

设计原则：**无状态、不依赖持久化存储**，任一实例可独立服务。

---

## 第六部分：前端设计

### 页面结构

| 页面 | URL | 用途 |
|------|-----|------|
| 首页 | `/` | IP 查询主工具页（IPv4 + IPv6 展示） |
| 隐私政策 | `/privacy` | GDPR 合规 |
| 什么是 IPv6 | `/docs/what-is-ipv6` | SEO 知识页 |
| IPv6 测试指南 | `/docs/ipv6-test-guide` | SEO 知识页 |

### UI 设计

- **主题**：深色/浅色自适应（`prefers-color-scheme` media query）
- **布局**：单列居中，最大宽度 640px
- **组件**：卡片式设计（`.card`），广告栏、IP 展示区、IPv6 状态区
- **交互状态**：5 态有限状态机（idle/loading/success/error/timeout/throttled），UI 与状态一一映射

### 前端工程

| 文件 | 职责 |
|------|------|
| `index.html` | 骨架 HTML，内联 CSS（`<style>`），SEO meta，Schema.org JSON-LD，_headers + _redirects |
| `js/i18n.js` | 国际化引擎，根据 `navigator.language` 自动选择 zh/en，`data-i18n` 属性驱动渲染 |
| `js/app.js` | 核心应用逻辑：IPv4 获取、IPv6 检测、60s 节流、复制、广告栏、状态管理 |

### 状态机

```
Idle → Loading → Success ──→ Idle（1.5s 后淡出）
               → Error
               → Timeout
               → Throttled（2s 后回到 Idle）
```

### 前端与后端通信

- **IPv4 获取**: `GET https://ip4.iohow.com/` + 头 `X-Client: web`，超时 5s
- **IPv6 检测**: `GET https://ip6.iohow.com/` + 头 `X-Client: web`，超时 8s
- **广告配置**: 通过响应头 `X-Ad-Enabled/X-Ad-Text/X-Ad-URL` 获取
- **节流**: 前端 60s 窗口，窗口内重复点击不发起请求，立即显示"已是最新结果"

---

## 第七部分：后端开发设计

### 项目结构

```
backend/
├── main.go              # 入口：初始化、双栈监听、优雅退出、slog 配置
├── config.go            # 配置加载/热加载、环境变量覆盖、语言检测、广告配置读取
├── handler.go           # HTTP Handler：/health, /readyz, /metrics, /ad-config, /all, /
├── middleware.go        # 11 层中间件链、IP 提取从 Context、日志、安全头、限流
├── ip_extract.go        # 真实 IP 提取、CF CIDR 热加载+兜底定时、cf_only 校验、可信代理支持
├── ratelimit.go         # 单 IP + 全局 Token Bucket 限流（可开关/选模式）
├── ad.go                # Web 客户端判断
├── geoip.go             # GeoLite2 City+ASN 查询、LRU 缓存、DB 热加载+配置热重载、中英文地名
├── metrics.go           # Prometheus 格式指标
├── errors.go            # 错误消息集中管理
├── monitor.go           # 内置自监控告警引擎
├── cache.go             # 泛型 LRU 缓存
├── main_test.go         # 测试（单元 + 集成）
├── config.yaml          # 默认配置文件
├── go.mod               # Go 模块定义
└── go.sum               # 依赖校验和
```

### 核心业务流程

```
main:main()
  │
  ├── LoadConfig(configPath) → 解析 YAML + 环境变量覆盖
  ├── NewIPExtractor(cfCidrPath, reloadInterval) → 加载 CF CIDR + fsnotify watcher + 兜底定时器
  ├── NewPerIPRateLimiter(...) → 单 IP 限流器 + cleanup goroutine
  ├── NewGlobalRateLimiter(...) → 全局限流器
  ├── NewMetrics() → 指标收集器
  ├── NewGeoIP(dbPath, asnDbPath, enabled) → GeoIP 查询器 + fsnotify watcher
  ├── NewMonitor(cfg, metrics) → 自监控引擎
  ├── http.NewServeMux → 注册路由
  ├── 11 层中间件链包装 handler
  ├── 3 x http.Server (v4, v6, metrics) → 并发 ListenAndServe
  ├── config.StartHotReload() → fsnotify 后台监听（回调重配反代 CIDR + GeoIP Configure）
  ├── ready.Store(true) → 对外就绪
  ├── 等待 signal / error
  ├── ready.Store(false) → 停止接收新请求
  ├── 三 server.Shutdown(shutdownCtx) → 优雅关闭
  └── wg.Wait() → 退出
```

### 错误处理

所有 HTTP 错误消息集中定义在 `errors.go`：

| 状态码 | 常量 | 消息 |
|--------|------|------|
| 400 | `errBadRequest` | `Bad Request: invalid request` |
| 400 | `errBodyNotAllowed` | `Bad Request: request body not allowed` |
| 403 | `errForbidden` | `Forbidden` |
| 404 | `errNotFound` | `Not Found` |
| 405 | `errMethodNotAllowed` | `Method Not Allowed` |
| 414 | `errURLTooLong` | `Bad Request: URL too long` |
| 429 | `errTooManyRequests` | `Too Many Requests: rate limit exceeded, please retry later` |
| 503 | `errServiceUnavailable` | `Service Unavailable` |

设计原则：准确严谨，不暴露内部细节（堆栈、文件路径、版本号）。

---

## 第八部分：安全设计

### 四层防御矩阵

| 层级 | 组件 | 措施 |
|------|------|------|
| L1 | Cloudflare CDN | WAF、DDoS 防护、边缘速率限制、TLS |
| L2 | Caddy/Nginx | 真实 IP 还原（trusted_proxies）、前置限流、安全响应头 |
| L3 | Go 应用 | 11 层中间件链、IP 信任链、Token Bucket 限流、黑名单、Panic 恢复、超时控制、并发安全 |
| L4 | 系统 | nftables（仅 CF CIDR）、systemd hardening |

### IP 信任链

**原则**：不信任任何客户端提供的代理头，除非来源 IP 属于已知可信 CIDR。

```
RemoteAddr → 来源 IP ∈ CF CIDR？
  ├─ 是 → 取 CF-Connecting-IP → 校验 → 用于限流/日志
  └─ 否 → 来源 IP ∈ TRUSTED_PROXY_CIDRS？
       ├─ 是 → 取 X-Forwarded-For 最右跳 → 校验
       └─ 否 → cf_only 开启则 403；否则取 RemoteAddr（直连）
```

### 限流策略

| 维度 | 速率 | Burst | 超出响应 |
|------|------|-------|----------|
| 单 IP | 10 req/min | 5 | `429` + `Retry-After: 6` |
| 全局 | 5000 req/s | 5000 | `429` + `Retry-After: 1` |
| 单 IP 并发 | 8 | — | `429` |

### 安全响应头

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`（防 clickjacking）
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Access-Control-Allow-Origin: *`（仅 GET + 无凭据）
- CSP（通过 `_headers`）：`default-src 'self'; connect-src 'self' https://ip4.iohow.com https://ip6.iohow.com; script-src 'self'; frame-ancestors 'none'`

### HTTP 超时控制

| 参数 | 值 | 防护目标 |
|------|----|----------|
| ReadHeaderTimeout | 5s | 慢头攻击 |
| ReadTimeout | 10s | body 读取超时 |
| WriteTimeout | 10s | slowloris 写攻击 |
| IdleTimeout | 60s | 连接复用上限 |
| MaxHeaderBytes | 1KB | 头部大小限制 |
| URL 长度 | >256B 拒绝 | 长 URL 资源耗尽 |

### systemd 安全加固

| 指令 | 作用 |
|------|------|
| `NoNewPrivileges=true` | 禁止子进程获取新权限 |
| `ProtectSystem=strict` | 只读文件系统 |
| `PrivateTmp=true` | 独立临时目录 |
| `PrivateDevices=true` | 禁止访问设备 |
| `ProtectKernelTunables=true` | 禁止修改内核参数 |
| `ProtectKernelModules=true` | 禁止加载内核模块 |
| `RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX` | 限制网络协议族 |
| `RestrictNamespaces=true` | 禁止创建 namespace |
| `SystemCallFilter=@system-service` | 限制系统调用 |
| `CapabilityBoundingSet=CAP_NET_BIND_SERVICE` | 仅允许绑定特权端口 |

### 已知安全风险

| 风险 | 说明 | 建议 |
|------|------|------|
| **CF_API_TOKEN 明文** | 写在 `/etc/ip-lookup/env` 文件 | 引入 `systemd-creds encrypt` 或 sops |
| **无客户端认证** | API 无需认证即可调用 | 当前有意开放，如需管控可增加 API Key |
| **metrics 端点** | 默认仅绑定 127.0.0.1:20013 | 保持不暴露公网 |
| **无外部密钥管理** | MaxMind License Key 同样明文 | 同上 |

---

## 第九部分：测试验证

### 测试策略

| 测试类型 | 框架 | 覆盖范围 |
|----------|------|----------|
| 单元测试 | Go `testing` + `httptest` | 每个 handler、中间件、工具函数 |
| 集成测试 | `httptest.NewServer` 全链测试 | 中间件链完整调用 |
| 参数化测试 | Table-driven tests | IP 脱敏、语言检测、XFF 解析、findBucket |

### 测试用例清单

| 测试函数 | 类型 | 覆盖内容 |
|----------|------|----------|
| `TestHealthHandler` | 单元 | /health 返回 200 OK |
| `TestReadyzHandler` | 单元 | /readyz 返回 200 OK |
| `TestRootHandlerPureIP` | 集成 | 纯 IP 响应，不含广告 |
| `TestRootHandlerWithAPIAd` | 集成 | API 广告文案正确拼接 |
| `TestRootHandlerWebClient` | 集成 | Web 客户端返回纯 IP 无广告 |
| `TestJSONAPI` | 集成 | JSON 格式、Content-Type、IP/Version 字段 |
| `TestAdConfigHandler` | 集成 | /ad-config 返回 JSON，多语言正确 |
| `TestRateLimiter` | 单元 | 单 IP 限流允/拒逻辑 |
| `TestGlobalRateLimiter` | 单元 | 全局限流允许多次 |
| `TestIPMasking` | 参数化 | IPv4 脱敏正确 |
| `TestRequestMethodCheck` | 单元 | POST 被拒绝返回 405 |
| `TestConfigDefaultValues` | 单元 | 默认值断言 |
| `TestMetrics` | 单元 | 指标递增正确 |
| `TestDetectLanguage` | 参数化 | zh/en 识别 |
| `TestParseXFF` | 参数化 | X-Forwarded-For 解析 |
| `TestFullMiddlewareChain` | 集成 | 全链测试 |
| `TestLoggingMiddlewareCapturesRejectedStatus` | 集成 | 黑名单过滤+指标 |
| `TestFindBucket` | 参数化 | 延迟桶索引 |
| `TestMonitorThresholds` | 单元 | P99 计算、阈值评估 |
| `TestBuildPayloadFiring` | 单元 | Alertmanager v4 firing 负载结构 |
| `TestBuildPayloadResolved` | 单元 | Alertmanager v4 resolved 负载结构 |
| `TestMonitorFiringAndResolved` | 集成 | 阈值触发->恢复全流程 webhook 推送 |
| `TestMonitorSendResolvedFalse` | 集成 | `send_resolved: false` 时不发 resolved |
| `TestMonitorMultipleWebhookTargets` | 集成 | 多 webhook 目标均收到推送 |
| `TestMonitorAuthHeader` | 集成 | Bearer 认证头正确注入 |
| `TestMonitorCooldown` | 集成 | 冷却期内不重复告警 |
| `TestWebhookConfigValidation` | 单元 | URL/Auth 校验、空 type 默认 Bearer |
| `TestWebhookSendResolvedDefault` | 单元 | `send_resolved` nil 默认 true |

### 执行方式

```bash
make test      # cd backend && go test ./... -v -count=1
make verify    # 门禁脚本：test + lint + docker build + systemd-analyze + 文件完整性
```

---

## 第十部分：编译打包

### 开发环境依赖

- Go 1.23+
- `golangci-lint`（可选，用于 lint）
- Docker（可选，用于容器构建）

### 构建流程

```bash
# 标准构建
cd backend && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=v1.0.0" -o ip-lookup .

# Docker 多阶段构建
docker build -t ip-lookup:latest -f docker/Dockerfile .

# Makefile
make build       # 构建 binary
make docker      # 构建 Docker 镜像
make verify      # 完整门禁
```

### Dockerfile 结构

```dockerfile
# 多阶段构建
FROM golang:1.23-alpine AS builder
  → go mod download
  → CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ip-lookup

FROM gcr.io/distroless/static-debian12:nonroot
  COPY --from=builder /out/ip-lookup /usr/local/bin/ip-lookup
  COPY config.yaml /etc/ip-lookup/config.yaml
  USER nonroot:nonroot
  ENTRYPOINT ["/usr/local/bin/ip-lookup", "-config", "/etc/ip-lookup/config.yaml"]
```

### 版本管理

- 版本号通过 `-ldflags="-X main.version=<version>"` 注入
- 遵循 SemVer
- 构建产物：单二进制 `ip-lookup`（~5MB，静态编译，无外部依赖）

---

## 第十一部分：部署上线

### 环境准备

| 要求 | 规格 |
|------|------|
| 服务器 | 单 VPS，1 核 512MB 起 |
| 操作系统 | Linux（Debian 12 / Ubuntu 22.04+ 推荐） |
| 依赖 | systemd（运行）、Go 1.23+（仅构建时需要） |
| 网络 | 公网 IPv4 + IPv6 双栈，80/443 端口 |
| CDN | Cloudflare 免费套餐 |

### 部署流程（默认：systemd 二进制）

**步骤 1：服务器初始化**

```bash
apt update && apt upgrade -y
systemctl enable --now nftables
nft -f deploy/nftables/cloudflare-only.nft
```

**步骤 2：构建**

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ip-lookup ./backend
```

**步骤 3：一键安装**

```bash
sudo bash scripts/install-systemd.sh ./backend/ip-lookup
```

该脚本自动执行：
1. 创建 `iplookup` 用户和组
2. 创建数据目录 `/var/lib/ip-lookup` 和日志目录 `/var/log/ip-lookup`
3. 安装二进制到 `/usr/local/bin/ip-lookup`
4. 安装默认配置到 `/etc/ip-lookup/config.yaml`（如已存在则保留）
5. 设置 `cap_net_bind_service` 兜底
6. 安装并启用 systemd 服务

**步骤 4：配置**

编辑 `/etc/ip-lookup/config.yaml` 或创建 `/etc/ip-lookup/env`：

```bash
# /etc/ip-lookup/env
GEOIP_ENABLED=true
MAXMIND_LICENSE_KEY=your_key_here
```

**步骤 5：安装 Web 服务器（Caddy 推荐）**

```caddy
ip4.iohow.com {
    bind 0.0.0.0
    tls {
        dns cloudflare {env.CF_API_TOKEN}
    }
    trusted_proxies static {
        import /etc/caddy/cloudflare-trusted.conf
    }
    reverse_proxy 127.0.0.1:8080
}

ip6.iohow.com {
    bind [::]
    tls {
        dns cloudflare {env.CF_API_TOKEN}
    }
    trusted_proxies static {
        import /etc/caddy/cloudflare-trusted.conf
    }
    reverse_proxy [::1]:8081
}
```

**步骤 6：Cloudflare CIDR 同步**

```bash
sudo bash deploy/scripts/install-cf-sync-cron.sh cron
```

**步骤 7：DNS 配置**

| 域名 | 类型 | 值 | 说明 |
|------|------|----|------|
| ip.iohow.com | A + AAAA | Cloudflare CDN Proxied | 主站静态页面 |
| ip4.iohow.com | A | 源站 IPv4 | IPv4 API |
| ip6.iohow.com | AAAA | 源站 IPv6 | IPv6 API |

**步骤 8：验证**

```bash
curl localhost:8080/health        # → OK
curl localhost:8080/              # → <your IP>
curl -H "Accept: application/json" localhost:8080/  # → JSON
curl localhost:20013/metrics       # → Prometheus 格式
systemd-analyze security ip-lookup  # → ≤ 3.0
```

### 配置说明

| 配置方式 | 优先级 | 示例 |
|----------|--------|------|
| `config.yaml` | 低（默认值） | `/etc/ip-lookup/config.yaml` |
| 环境变量 | 高（覆盖 YAML） | `/etc/ip-lookup/env` → `RATE_PER_IP=30` |

### 回滚方案

```bash
# systemd 回滚
cp /usr/local/bin/ip-lookup.bak /usr/local/bin/ip-lookup
systemctl restart ip-lookup

# Docker 回滚
docker pull ip-lookup:previous-tag
docker compose up -d

# DNS 回滚（蓝绿部署）
Cloudflare DNS 切换到旧源站 IP
```

---

## 第十二部分：运维管理

### 服务管理

```bash
systemctl start|stop|restart|status ip-lookup
journalctl -u ip-lookup -f --since "10 min ago"
```

### 日志管理

| 项 | 说明 |
|----|------|
| 路径 | `/var/log/ip-lookup/ip-lookup.log` |
| 格式 | JSON 结构化（`log/slog` JSONHandler） |
| 轮转 | lumberjack: 50MB/文件，保留 7 份，30 天，gzip 压缩 |
| 级别 | debug/info/warn/error（配置项 `log_level`） |

日志字段（示例）：
```json
{"time":"2026-07-23T10:00:00Z","level":"INFO","msg":"request","ip":"192.168.1.0","method":"GET","path":"/","status":200,"latency_ms":3,"request_id":"a1b2c3d4","ua":"curl/8.0"}
```

### 监控方案

| 指标 | 端点/方式 | 用途 |
|------|-----------|------|
| 存活检查 | `/health` | 负载均衡/探针 |
| 就绪检查 | `/readyz` | 优雅退出时返回 503 |
| Prometheus 指标 | `127.0.0.1:20013/metrics` | 请求量/限流/状态码/延迟 |
| 自监控告警 | 内置引擎 → | 自监控告警 | 内置引擎 → Alertmanager v4 Webhook | 错误率/P99/限流率超阈值推送 |

### 告警机制

| 异常类型 | 触发条件 | 通知方式 |
|----------|----------|----------|
| 错误率过高 | 5xx 占比 > 5% | Alertmanager v4 Webhook |
| P99 延迟过高 | P99 > 2000ms | Alertmanager v4 Webhook |
| 限流命中率过高 | > 10% 请求被限流 | Alertmanager v4 Webhook |

> Webhook 负载遵循 Alertmanager v4 规范，支持多目标推送、Bearer 认证、`send_resolved` 恢复通知。详见 [运维手册 - 自监控告警](operation.md#自监控告警)。

### 备份恢复

```bash
# 备份（配置 + GeoIP）
tar czf /var/backups/ip-lookup-$(date +%Y%m%d).tar.gz /etc/ip-lookup/ /var/lib/ip-lookup/

# 恢复
tar xzf /var/backups/ip-lookup-20260723.tar.gz -C /
```

### 定期维护任务

| 频率 | 任务 | 自动化 |
|------|------|--------|
| 每日 03:00 | CF CIDR 全量同步 | ✅ cron |
| 每 6h | CF CIDR 增量校验 | ✅ cron/systemd timer |
| 每周日凌晨 4:00 | GeoIP 数据库更新 | ✅ cron |
| 按需 | 广告文案更新（编辑 config.yaml） | ✅ fsnotify 热加载 |
| 按需 | 限流参数调整（编辑 env） | 需 `systemctl restart` |
| 每月 | 检查 Go 版本更新 | 建议 Dependabot |
| 每季度 | 完整性能评估 | 手动 |

---

## 第十三部分：问题排查手册

### 启动失败

| 问题 | 现象 | 排查步骤 | 解决方案 |
|------|------|----------|----------|
| 端口被占用 | `bind: address already in use` | `ss -tlnp \| grep 8080` | 停用占用服务或修改配置端口 |
| config.yaml 错误 | `failed to load config: ...` | `journalctl -u ip-lookup -n 20` | 检查 YAML 格式，本地验证 |
| 日志目录权限 | 启动成功但日志不写入 | `ls -la /var/log/ip-lookup/` | `chown iplookup:iplookup /var/log/ip-lookup/` |
| 广告 URL 格式无效 | 校验失败拒绝启动 | 检查 `api_ad_url_*` 和 `web_ad_url_*` | 确保 URL 以 `http://` 或 `https://` 开头 |

### 运行时异常

| 问题 | 现象 | 排查 | 解决方案 |
|------|------|------|----------|
| 限流误伤 | 正常用户收到 429 | `journalctl \| grep rate_limit_hit` | 调大 RATE_PER_IP 或 RATE_GLOBAL |
| GeoIP 不生效 | JSON API 无 city/country | 检查 `geoip_enabled`、DB 文件存在 | 开启 geoip 并运行 update-geoip.sh |
| 伪造 IP 绕过限流 | 限流不生效 | 检查 cf-cidrs.txt 是否最新 | 运行 update-cloudflare-ip.sh |
| 内存增长异常 | RSS 持续增长 | `/metrics` 查看 inflight_requests | 检查 connLimit 和 rateLimiter 清理是否正常 |
| Webhook 告警未送达 | `journalctl \| grep "monitor: webhook"` | 检查 `webhook_configs[].url` 可达性、Bearer token 正确性 | 确保 URL 为 `https://`，`authorization.type` 仅支持 `Bearer` |
| 告警未触发 | `journalctl \| grep "self-monitoring"` 无 WARN | 检查 `monitoring.enabled` 是否为 `true`、阈值是否合理 | 确认 `check_interval` 内有足够流量产生 delta |

### 网络问题

| 问题 | 现象 | 排查 | 解决方案 |
|------|------|------|----------|
| IPv6 不可达 | 前端 IPv6 检测一直 loading | `curl -6 https://ip6.iohow.com/` | 检查源站 IPv6 连通性和 DNS |
| CDN 未命中 | 公网请求绕过 Cloudflare | `curl -v https://ip4.iohow.com/ \| grep cloudflare` | 检查 DNS 是否为 Proxied |
| 防火墙误拦 | 正常流量被拒绝 | `nft list ruleset` | 检查 cloudflare-only.nft 是否包含所有 CF CIDR |

### 性能问题

| 问题 | 现象 | 排查 | 解决方案 |
|------|------|------|----------|
| CPU 高 | 100% CPU | `/metrics` 查 qps 和延迟分布 | 检查全局限流、是否被扫描攻击 |
| 延迟高 | 响应 >200ms | 检查 GeoIP 是否启用、限流是否触发 | 关闭 GeoIP、调大限流参数 |
| 连接数超限 | 429 频繁 | `ss -tnp \| grep 8080 \| wc -l` | 调大 `MAX_CONNS_PER_IP` |

---

## 第十四部分：运营维护

### 运营指标

| 指标 | 采集方式 |
|------|----------|
| DAU/MAU | Cloudflare Web Analytics（无 Cookie） |
| API 调用量 | `/metrics` → `http_requests_total` |
| 限流命中率 | `rate_limit_hits_total / http_requests_total` |
| P99 延迟 | `http_request_duration_ms_bucket{le="..."}` 桶计算 |
| 错误率 | `http_requests_5xx_total / http_requests_total` |

### 版本升级流程

```bash
git pull / checkout new tag
make build
systemctl stop ip-lookup
cp backend/ip-lookup /usr/local/bin/ip-lookup
systemctl start ip-lookup
curl localhost:8080/health  # → OK
curl localhost:8080/         # → IP
```

---

## 第十五部分：项目总结

### 项目成果

| 维度 | 成果 |
|------|------|
| **代码** | Go 后端 15 个源文件（~2000 行）、前端 3 个文件（~500 行）、部署脚本 5 个、文档 13 篇 |
| **架构** | 无状态、无数据库、单二进制部署、四层纵深防御、11 层中间件链、双栈分离监听 |
| **可观测性** | 内置 Prometheus 指标 + 自监控告警引擎 + 结构化日志轮转 |
| **运营** | 运维成本趋近于零，服务器成本 <$10/月，自动 CF CIDR 同步 + GeoIP 更新 |

### 技术亮点

1. **最小信任 IP 提取**：基于 CIDR 校验的信任链，拒绝伪造代理头
2. **三重 fsnotify 热加载**：Config + IPExtractor + GeoIP 均自动热加载，零重启
3. **16 分片连接计数器**：消除全局 Mutex，支撑 10K+ 并发
4. **sync.Map Per-IP 限流器**：原子 LoadOrStore 替代 RWMutex
5. **Context 传递真实 IP**：前置提取一次，消除后续 4 次重复 CIDR 扫描
6. **GeoIP LRU 缓存**：10K 条目，重复查询命中率 ~80%
7. **双栈独立 http.Server**：DNS 层天然过滤错误协议族访问
8. **内置自监控**：不依赖外部监控，独立检测阈值并通过 Alertmanager v4 Webhook 推送告警（支持多目标 + Bearer 认证 + 恢复通知）

### 当前限制

| 限制 | 说明 | 影响评估 |
|------|------|----------|
| **无 CI/CD 自动化** | 仅有 PR verify，无 auto-build + release | 部署手动，版本不可追溯 |
| **密钥明文存储** | CF_API_TOKEN、MAXMIND_LICENSE_KEY 明文 | 中等风险 |
| **单实例架构** | 水平扩展需手动改 DNS | 当前流量无瓶颈 |
| **无 staging 环境** | 变更直接上生产 | 中等风险 |

### 已知问题

| 问题 | 严重度 | 状态 |
|------|--------|------|
| 日志 truncateUA 仅截断未 sanitize | 低 | 不影响安全 |
| 广告 URL 校验仅检查 scheme | 低 | 不校验域名合法性 |
| Docker Caddy sidecar 完全注释 | 低 | 需手动解除注释 |

### 版本路线图

| 版本 | 计划内容 |
|------|----------|
| **V1.0（当前）** | 核心 IP 查询、双栈、GeoIP、广告配置、自监控、完整文档 |
| **V1.1（短期）** | CI release pipeline（GitHub Actions tag → build → Docker push + GH Release）、密钥管理（sops） |
| **V1.2（中期）** | 子网计算工具页、批量 IP 查询 API、PWA manifest + Service Worker |
| **V2.0（长期）** | 多 region 部署（可选 K8s）、API Key 认证、企业级付费计划、日/韩/法/德语国际化 |

---

> 文档完整。验证状态：`make verify` 已通过，系统达到生产就绪标准。
