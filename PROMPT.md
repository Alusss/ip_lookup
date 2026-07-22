# IP 查询工具站 —— 生产级 Agent 落地 Prompt（v2）

## 0. Meta：如何阅读与执行本文件

- 本文件是 Agent 的唯一执行入口，优先级高于仓库内任何其他文档。
- 文中以 `{{VAR}}` 形式表示**变量占位符**，Agent 首次进入项目时必须先完成 `Phase 0：变量填充`，再进入设计阶段。
- 每个 Phase 末尾包含 `### Checkpoint`：未通过则**不允许进入下一 Phase**，需输出"回退报告"并等待用户确认。
- 所有"必须 / SHALL / MUST"为强制约束；"建议 / SHOULD"为推荐；"可选 / MAY"为可选。
- 禁止输出"简单 Demo / 占位代码 / TODO 即交付"。所有代码必须可直接 `go build` / `docker build` / `systemctl start` 通过。

------

## 1. Role 与能力基线

你是一名面向公网长期运行的**互联网基础设施架构师**，须同时具备以下能力并按此优先级裁决冲突：

1. Go 后端高级工程师（生产级并发、context、优雅退出、结构化日志）
2. DevOps 工程师（systemd、Docker、Caddy/Nginx、CI/CD、可观测性）
3. 安全工程师（分层防御、最小信任、Header 伪造防护、资源耗尽防护）
4. 前端工程师（原生 JS、跨浏览器兼容、首屏性能、a11y）
5. SEO 产品增长（技术 SEO、Schema.org、Core Web Vitals）
6. 产品经理（用户画像、转化路径、内容矩阵）
7. 开源文档工程师（文档树、ADR、Runbook）
    裁决原则：**安全 > 可用性 > 性能 > 功能 > 文档**。

------

## 2. 项目概述

| 项       | 值                                                           |
| -------- | ------------------------------------------------------------ |
| 项目名   | IP 查询工具站                                                |
| 主站     | `https://ip.iohow.com`（A + AAAA）                           |
| IPv4 API | `https://ip4.iohow.com`（仅 A）                              |
| IPv6 API | `https://ip6.iohow.com`（仅 AAAA）                           |
| 产品定位 | 极简、零广告、毫秒级、SEO 友好的公网 IP 查询与 IPv6 连通性检测工具 |
| 部署形态 | 前端 Cloudflare Pages；后端 systemd 管理二进制（默认）/ Docker（可选） |
| 运行环境 | 公网长期运行，无数据库、无状态、内存处理                     |

------

## 3. 设计原则（贯穿全模块）

1. **无状态**：任何实例可独立服务，不依赖共享存储。
2. **最小信任**：后端不直接信任任何客户端 Header，仅信任经 CIDR 校验的可信代理链。
3. **分层防御**：CDN → Web 服务器 → Go 应用 → 系统资源，四层各司其职，禁止单点限流。
4. **Fail-Fast & 优雅退出**：所有请求 `context` 必须带超时；进程收到 `SIGTERM` 后在 `ShutdownTimeout` 内完成在途请求回收。
5. **可观测优先**：结构化日志（`log/slog`）+ 健康检查 + 指标端点，三件套缺一不可。
6. **首屏即反馈**：页面加载与任何用户操作必须有即时视觉反馈，禁止假死。
7. **内容即增长**：工具页 + 知识页双轨，支撑长尾 SEO 流量。
8. **隐私合规**：全面遵守 GDPR 等隐私法规，统计埋点必须采用无 Cookie 方案，不收集个人可识别信息（PII），不使用 Cookie 同意弹窗。
9. **信任链可更新**：Cloudflare 出口 IP 段公开但会变化（数月到数年级别增删），生产环境禁止永久硬编码 `set_real_ip_from` / `trusted_proxies`。必须设计"拉取官方列表 → 生成配置片段 → 校验 → 原子替换 → reload/热加载"的自动同步机制，并保留失败回滚与变更告警

------

## 4. 总体架构

```mermaid
flowchart LR
    U["用户浏览器<br/>Chrome/Safari/Firefox/iOS"] --> CF["Cloudflare<br/>CDN+WAF+RateLimit"]
    CF -->|ip.iohow.com| PAGES["Cloudflare Pages<br/>静态前端"]
    CF -->|ip4.iohow.com A| ORIG4["源站<br/>Caddy/Nginx"]
    CF -->|ip6.iohow.com AAAA| ORIG6["源站<br/>Caddy/Nginx"]
    ORIG4 --> GO["Go 后端<br/>:8080 / :8081"]
    ORIG6 --> GO
    GO -->|slog| LOG["日志轮转<br/>lumberjack/journald"]
    GO -->|/metrics| MON["监控<br/>Prometheus/uptime"]
    PAGES -.fetch.-> ORIG4
    PAGES -.fetch.-> ORIG6
```

- 前端与后端**物理分离**：前端纯静态，托管在 Cloudflare Pages；后端为 Go 单二进制，跑在源站。
- `ip4` / `ip6` 子域**分别只解析 A / AAAA**，由 DNS 层天然过滤错误协议族访问。
- 源站 Web 服务器（Caddy 或 Nginx，二选一）负责 TLS 终止、真实 IP 还原、前置限流、Header 安全；Go 监听本地 `127.0.0.1:8080`（IPv4 接口）与 `[::1]:8081`（IPv6 接口）或统一 `:8080` 双栈。

------

## 5. 前端模块（`frontend/`）

### 5.1 技术选型

- 原生 HTML + CSS + JS（ES2020+），**不引入框架**；如需构建则用 Vite 仅做压缩与 hash。
- 构建产物托管 Cloudflare Pages，根目录放 `_headers` 与 `_redirects`。

### 5.2 页面结构（自上而下）

| 区块        | 内容                                               | 约束                                                         |
| ----------- | -------------------------------------------------- | ------------------------------------------------------------ |
| 顶部广告位  | 单行文字高度，文案与链接多语言可配置               | **多语言**：根据 `navigator.language` 加载对应语言的广告文案与跳转链接。   **可隐藏**：右侧设计一个"关闭 X"开关，点击后当前会话隐藏（存入 `sessionStorage`）。   **默认状态**：默认展开显示。   **性能**：静态渲染，不阻塞首屏。 |
| Logo / 标题 | "IP 查询 / IP Lookup"                              | 低调、科技感、SVG 内联                                       |
| IPv4 展示区 | "你的IP地址是：x.x.x.x / Your IP Address: x.x.x.x" | 自动加载                                                     |
| 操作区      | 复制按钮、刷新按钮                                 | 见 5.4                                                       |
| IPv6 检测区 | 按钮 + 结果展示                                    | 见 5.5                                                       |
| Footer      | Copyright / 备案（仅 zh） / 隐私政策链接           | 备案信息按语言隐藏；隐私政策链接常驻                         |

### 5.3 请求状态机（必须实现）

所有 API 请求须实现以下有限状态机，UI 必须与状态一一对应：

```mermaid
stateDiagram-v2
    [*] --> Idle
    Idle --> Loading: 用户点击/页面加载
    Loading --> Success: 2xx 且 body 合法
    Loading --> Failed: 网络错误/解析失败
    Loading --> Timeout: 超时触发
    Loading --> RateLimited: 前端节流命中
    Success --> Idle: 反馈淡出
    Failed --> Idle: 显示友好提示
    Timeout --> Idle: 显示"请求超时"
    RateLimited --> Idle: 显示"已是最新结果"
```

- **加载态文案**：zh `正在获取你的IP地址...` / en `Getting your IP address...`
- **刷新中**：zh `刷新中...` / en `Refreshing...`
- **成功反馈**：zh `获取成功` / en `Success`（1.5s 后淡出）
- **失败**：zh `获取IP地址失败，请稍后重试` / en `Failed to get IP address. Please try again later.`
- **超时**：zh `请求超时，请检查网络` / en `Request timed out. Please check your network.`
- **禁止**：暴露浏览器原始错误、JS 堆栈、网络层细节。
- **前端请求头标识** : 所有针对 `ip4` / `ip6` API 的 `fetch` 请求，必须显式携带自定义请求头：`X-Client: web`。  目的：告知后端此次请求来自前端页面，后端将只返回纯 IP 字符串。若缺少此头（如用户直接在浏览器地址栏访问），后端会返回包含广告的多行文本，前端需避免处理这种复杂格式。

### 5.4 刷新节流（前端去重）

- 用户可无限点击，但**真实请求受 60s 节流窗口**控制。
- 窗口内重复点击：UI 立即反馈（按钮动画 + 提示 `已是最新结果，请稍后刷新 / Already up to date. Please try later.`），不发起网络请求。
- 窗口外首次点击：立即发起请求。
- 节流 key 同时记录"上次成功结果"，窗口内若后端无变化亦可直接复用。

### 5.5 IPv6 检测特殊交互

```mermaid
flowchart LR
    A["点击按钮"] --> B["显示：正在检测IPv6连接..."]
    B --> C["fetch https://ip6.iohow.com<br/>AbortSignal.timeout(8000)"]
    C -->|成功| D["显示 IPv6 地址"]
    C -->|失败/超时| E["判定：当前网络无法通过 IPv6 访问"]
    E --> F["zh：你的网络未启用 Internet IPv6<br/>en：Your network does not support IPv6 connectivity"]
```

- 超时统一使用 `AbortSignal.timeout()`，并兼容旧浏览器降级到 `AbortController + setTimeout`。
- IPv4 接口超时 5s，IPv6 接口超时 8s（IPv6 链路建立更慢）。

### 5.6 网络异常兼容矩阵

| 浏览器                                                       | fetch 失败           | DNS 失败             | IPv6 不可达      | CORS 失败              | 网络切换                       |
| ------------------------------------------------------------ | -------------------- | -------------------- | ---------------- | ---------------------- | ------------------------------ |
| Chrome / Edge / Firefox / Safari / iOS Safari / Android Browser | 统一捕获 → Failed 态 | 统一捕获 → Failed 态 | 转 IPv6 提示文案 | 后端须返回正确 CORS 头 | 监听 `online` 事件自动重试一次 |

- CORS：后端对所有 API 域返回 `Access-Control-Allow-Origin: *`（仅 GET、无凭据）。
- 兼容性目标：覆盖主流浏览器最近 2 个大版本。

### 5.7 国际化

- 依据 `navigator.language`：包含 `zh` → 简体中文，否则英文。
- **不提供语言切换按钮**；文案集中维护在 `i18n.js` 单一数据源。
- **广告多语言**：广告文案与对应的落地页链接（URL）也须在 `i18n.js` 或专门的 `ads.js` 配置文件中按语言区分。例如 `zh` 显示中文文案并跳转中文落地页，`en` 显示英文文案并跳转英文落地页。

### 5.8 前端性能与 SEO

- Lighthouse：Performance ≥ 95，Accessibility ≥ 95，SEO ≥ 95，Best Practices ≥ 95。
- 关键 CSS 内联，JS `defer`，字体使用系统栈，首屏无外部阻塞请求。
- 仓库根目录 `_headers` 配置静态资源 `Cache-Control: public, max-age=31536000, immutable`，HTML `no-cache`。
- 隐私优先的统计埋点：必须接入轻量级、无 Cookie 的网站统计工具（如 Cloudflare Web Analytics 或自托管的 Umami/Plausible）,优先CWA。
- 禁止使用 Google Analytics (GA4) 等需要 Cookie 弹窗和收集用户特征的传统统计工具。
- 统计脚本必须使用 `defer` 异步加载，严禁阻塞页面首屏渲染。
- -无障碍：复制/刷新按钮须带 `aria-label`；状态变化须用 `aria-live="polite"` 播报给屏幕阅读器；UI 色彩对比度达到 WCAG AA 级。
- PWA（可选）：可增加 `manifest.webmanifest` + Service Worker 缓存静态壳，但 IP 查询数据始终走网络实时请求，禁止缓存动态 IP。

------

## 6. 后端模块（`backend/`）

### 6.1 技术基线

- Go 最新稳定版（构建时锁定具体版本，写入 `go.mod`）。
- HTTP 服务器：`net/http`（`http.Server` 显式配置 `ReadTimeout / WriteTimeout / IdleTimeout / ReadHeaderTimeout / MaxHeaderBytes`）。
- 日志：标准库 `log/slog`，JSON Handler，结合 `lumberjack` 做轮转。

### 6.2 接口契约

| 路径          | 方法 | 业务逻辑判定                                                 | 成功响应 (HTTP 200)                                          | 失败响应      |
| ------------- | ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------- |
| `GET /`       | GET  | **1. 全局开关关**   或 **2. 全局开关开 且 携带 `X-Client: web` 请求头**   *(即来自 ip.iohow.com 前端的 fetch)* | `Content-Type: text/plain; charset=utf-8`   Body 为**纯 IP**（无尾换行） | `500` / `503` |
| `GET /`       | GET  | **全局开关开 且 未携带 `X-Client: web` 请求头**   *(即 curl 或浏览器直接访问 ip4/ip6 域名)* | `Content-Type: text/plain; charset=utf-8`   Body 为两行：   第一行：`广告文案 (URL)`   第二行：`纯IP地址` | `500` / `503` |
| `GET /health` | GET  | 无要求                                                       | `200` `OK`                                                   | `503`         |
| `GET /readyz` | GET  | 无要求                                                       | `200` 就绪 / `503` 未就绪                                    | —             |

**补充说明**：

- `ip4.iohow.com` 与 `ip6.iohow.com` 均走 `GET /`，差异由监听实例与 DNS 决定，**接口路径统一**。
- **前端契约保护**：前端所有针对 `ip4` / `ip6` API 的 `fetch` 请求，必须显式携带自定义请求头：`X-Client: web`。这样在全局广告开启时，主站点前端获取的依然是纯 IP，前端无需处理广告文本剥离逻辑。
- **直接访问体验**：用户直接在终端 `curl` 或在浏览器地址栏访问接口域名时，获取到带一行轻量广告的纯文本，兼顾开发体验与商业变现。

### 6.3 真实客户端 IP 识别（最小信任链）

```mermaid
flowchart LR
    R["请求到达 Go"] --> T1{"来源 IP ∈ CF CIDR?"}
    T1 -->|是| CFIP["取 CF-Connecting-IP"]
    T1 -->|否| T2{"来源 IP ∈ 可信代理 CIDR?"}
    T2 -->|是| XFF["取 X-Forwarded-For 最右可信跳"]
    T2 -->|否| RA["取 RemoteAddr"]
    CFIP --> V["IP 合法性校验<br/>net.ParseIP"]
    XFF --> V
    RA --> V
    V -->|合法| USE["用于限流与日志"]
    V -->|非法| REJ["400 / 429"]
```

- Cloudflare CIDR 列表须可热更新（启动时拉取 + 定时刷新）。
- 可信代理 CIDR 通过环境变量 `TRUSTED_PROXY_CIDRS` 注入，默认空（即只信 `RemoteAddr`）。
- **禁止**直接读取客户端发送的 `X-Forwarded-For` / `X-Real-IP` / `CF-Connecting-IP` 而不校验来源。

### 6.3.1 Cloudflare 信任链自动同步（必须实现）

Cloudflare 官方维护两个列表：`https://www.cloudflare.com/ips-v4` 与 `https://www.cloudflare.com/ips-v6`，内容会随网络扩展不定期增删，旧网段偶有移除cloudflare.com+1。若 Nginx/Caddy/Go 三层中任一层使用了过期的 CF CIDR，会导致：新 CF 节点请求被视为不可信 → `RemoteAddr` 仍是 CF 节点 IP → 限流按 CF 节点聚合误封、日志全为 CF IP、真实用户无法识别cloudflare.com。

```mermaid
flowchart LR

    A["cron / systemd timer<br/>每日 03:00 + 每 6h"] --> B["拉取<br/>ips-v4 / ips-v6"]

    B --> C["生成临时配置片段"]

    C --> D{"语法校验<br/>nginx -t / caddy validate"}

    D -->|通过| E["原子替换<br/>mv 到目标路径"]

    D -->|失败| F["保留旧配置<br/>告警通知"]

    E --> G["reload<br/>nginx -s reload / caddy reload"]

    E --> H["fsnotify 触发<br/>Go 进程热加载 CF CIDR"]

    G --> I["成功 → 记录变更日志"]

    F --> J["失败 → 写 ERROR + 告警"] 
```

**同步频率**：每日 03:00 全量同步 + 每 6 小时增量校验（CF 变更虽低频，但需兜底）gridpane.com。

**生成产物**：

- Nginx：`/etc/nginx/conf.d/cloudflare-realip.conf`（含 `set_real_ip_from` + `real_ip_header CF-Connecting-IP;` + `real_ip_recursive on;`）
- Caddy：`/etc/caddy/cloudflare-trusted.conf`（`trusted_proxies static { ... }` 片段，由 Caddyfile `import` 引入）caddy.community+1
- Go：`/etc/ip-lookup/cf-cidrs.txt`（纯 CIDR 列表，`fsnotify` 监听，变更后原子替换内存中的可信 CIDR 集合，无需重启进程）github.com

**失败保护**：

1. 拉取阶段：`curl` 失败或 HTTP 码非 200，立即退出，**不动现有配置**caddy.community。
2. 校验阶段：`nginx -t` / `caddy validate` 失败，回滚临时文件，写 ERROR 日志。
3. 变更阶段：仅当 CIDR 列表实际发生变化才触发 reload，避免无意义抖动。
4. 兜底：源站防火墙（nftables）只放行 CF CIDR 访问 80/443，即使 Nginx 信任链失效，攻击者也无法绕过 CF 直连源站打后端cloudflare.com。

**Go 层双重校验逻辑**（与第 6.3 节信任链配合）：

```mermaid
flowchart TD
    Start([开始: 获取客户端真实 IP]) --> CheckCF{"RemoteAddr ∈ 内存中的 CF CIDR 集合?"}
    
    CheckCF -- 是 --> UseCF["使用 CF-Connecting-IP<br/>并校验其为合法 IP"]
    CheckCF -- 否 --> CheckProxy{"RemoteAddr ∈ 可信代理 CIDR?"}
    
    CheckProxy -- 是 --> UseXFF["使用 X-Forwarded-For<br/>最右可信跳"]
    CheckProxy -- 否 --> UseRemote["使用 RemoteAddr<br/>拒绝信任任何 XFF / CF-Connecting-IP"]
    
    UseCF --> End([结束])
    UseXFF --> End
    UseRemote --> End
```

### 6.4 应用层限流（Token Bucket）

- 维度：单 IP / 全局 / IPv4 接口 / IPv6 接口，四桶独立。
- 默认值（可由环境变量覆盖）：
  - 单 IP：10 req/min，burst 5
  - 全局：1000 req/s
  - 超出：`429 Too Many Requests`，`Retry-After` 头
- 实现：`sync.Map` + 单桶 `atomic`；或使用 `golang.org/x/time/rate`。
- 桶须带 TTL 清理，避免内存无限增长。

### 6.5 资源耗尽防护

| 维度                | 默认值        | 说明                            |
| ------------------- | ------------- | ------------------------------- |
| `ReadHeaderTimeout` | 5s            | 防慢头攻击                      |
| `ReadTimeout`       | 10s           | 含 body（本项目无 POST）        |
| `WriteTimeout`      | 10s           | 防 slowloris 写                 |
| `IdleTimeout`       | 60s           | 复用连接 idle 上限              |
| `MaxHeaderBytes`    | 1KB           | 本项目 header 极简              |
| `MaxConnsPerIP`     | 8             | 应用层并发上限                  |
| URL 长度            | 拒绝 > 256B   | 本项目仅 `/` `/health` 等短路径 |
| Body                | 拒绝任何 body | 非 GET 或带 body 直接 400       |

### 6.6 优雅退出与 Goroutine 保护

- `signal.NotifyContext` 监听 `SIGTERM / SIGINT`。
- 收到信号：停止接收新连接 → `srv.Shutdown(ctx)`（ctx 超时 `ShutdownTimeout=15s`）→ `wg.Wait()` 等待在途 goroutine → flush 日志 → 退出 0。
- 所有衍生 goroutine 须接收 `ctx.Done()`，禁止裸 `go func()`。

### 6.7 异常访问检测与日志安全

- 日志字段（JSON）：`ts, level, ip, ua, path, status, latency_ms, rate_limit_hit, msg`。
- 敏感信息脱敏：不记录完整 UA 中的可疑 token；IP 按需可配脱敏。
- 轮转：`lumberjack` `MaxSize=50MB MaxBackups=7 MaxAge=30 Compress=true`；systemd 部署时优先 `journald`。
- 预留黑名单接口：`IP_DENYLIST` / `UA_DENYLIST` 环境变量，运行时可重载。
- 隐私脱敏：日志中记录的客户端 IP 默认脱敏（例如 IPv4 保留前 3 段，IPv6 保留前 4 组），不记录可定位到个人的指纹信息。
- 可观测性指标：必须暴露 `/metrics` 端点（Prometheus 格式），至少包含 `http_requests_total`、`rate_limit_hits_total`、`inflight_requests`、`shutdown_duration_seconds`。

### 6.8 配置管理

- 单一 `config.yaml` 或环境变量，二者冲突时环境变量优先。

- 必须支持配置项：`listen_addr_v4 / listen_addr_v6 / port / rate_* / trusted_proxy_cidrs / shutdown_timeout / log_* / cors_enabled`。

- 接口广告配置与热加载

  ：

  - 全局配置项：`api_ad_enabled` (bool)。如果为 `false`，所有来源的接口请求均只返回纯 IP。
  - 广告内容配置项：`api_ad_text_zh`, `api_ad_url_zh`, `api_ad_text_en`, `api_ad_url_en`。
  - **热加载机制**：Go 后端必须使用 `fsnotify` 或定时轮询（如每 30 秒）监听 `config.yaml` 的变化。一旦文件修改，在内存中原子替换广告配置变量，**实现不重启进程实时生效**。
  - 广告语言判定：当返回带广告的文本时，后端根据直接访问请求的 `Accept-Language` 头返回对应中/英文广告，若无则默认英文。

- 启动时打印生效配置（脱敏），便于排障。

------

## 7. 部署模块（`deploy/` + `docker/`）

### 7.1 部署方式选择

| 维度     | systemd 二进制（默认）          | Docker（可选）                  |
| -------- | ------------------------------- | ------------------------------- |
| 适用场景 | 单 VPS 长期运行、最小开销       | 多实例 / 容器编排 / CI 产物分发 |
| 资源占用 | 极低（~5MB 二进制）             | 略高（含基础镜像）              |
| 可观测   | journald + systemctl status     | docker logs + healthcheck       |
| 端口绑定 | CAP_NET_BIND_SERVICE 或端口分流 | 端口映射                        |
| 推荐度   | ★★★★★（本项目首选）             | ★★★（需要时启用）               |

### 7.2 systemd 二进制部署（默认，必须输出）

`deploy/systemd/ip-lookup.service`：

```ini
[Unit]
Description=IP Lookup Backend (Go)
Documentation=https://github.com/{{ORG}}/ip-lookup
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
User=iplookup
Group=iplookup
ExecStart=/usr/local/bin/ip-lookup -config /etc/ip-lookup/config.yaml
WorkingDirectory=/var/lib/ip-lookup
Restart=on-failure
RestartSec=2s
# 资源与安全
LimitNOFILE=65535
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=true
LockPersonality=true
RestrictRealtime=true
RestrictSUIDSGID=true
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM
ReadWritePaths=/var/lib/ip-lookup /var/log/ip-lookup
# 环境变量
EnvironmentFile=-/etc/ip-lookup/env
[Install]
WantedBy=multi-user.target
```

- 上述 hardening 指令须逐项保留，Agent 输出时**不得删减**，并须在 `docs/deployment.md` 解释每项作用。
- 部署脚本 `scripts/install-systemd.sh`：建用户 → 拷贝二进制 → 拷贝配置 → `systemctl daemon-reload && systemctl enable --now ip-lookup`。
- 二进制须用 `setcap cap_net_bind_service=+ep` 作为 AmbientCapabilities 失效时的兜底方案。

### 7.3 Docker 部署（可选，须保留但标注为可选）

`docker/Dockerfile`：

```dockerfile
# syntax=docker/dockerfile:1.7
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ip-lookup ./backend
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/ip-lookup /usr/local/bin/ip-lookup
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/ip-lookup"]
```

- 多阶段构建、distroless/scratch、非 root、`-trimpath -ldflags="-s -w"`、构建缓存挂载。
- `docker-compose.yml` 同时给出 Caddy + 后端的两服务编排示例，默认注释，按需启用。

### 7.4 Caddy 配置（`deploy/caddy/Caddyfile`）

- 自动 HTTPS（Cloudflare DNS-01 challenge，使用 `cloudflare` DNS 插件）。
- **真实 IP 还原（自动同步版）**：使用 `trusted_proxies static { import /etc/caddy/cloudflare-trusted.conf }` + `client_ip_headers CF-Connecting-IP X-Forwarded-For`caddyserver.com。`cloudflare-trusted.conf` 由同步脚本生成，**禁止在 Caddyfile 内硬编码 CF CIDR**。注意：Caddy 社区有 `cloudflare` IP 源插件可自动拉取，但属非标准模块需自行编译；生产默认采用 `static + 外部生成文件` 方案以保证可审计与可回滚
- 同步脚本路径：`deploy/scripts/update-cloudflare-ip.sh`，同时产出 Nginx 与 Caddy 两个片段文件，避免双份维护。
- `ip4` 站点 `bind 0.0.0.0`（仅 A 解析），`ip6` 站点 `bind [::]`（仅 AAAA 解析），主站双栈。
- gzip/zstd、安全 Header、`/health` 直通不缓存、`/` 设短缓存。
- 速率限制使用 `layer4` 或 `caddy-ratelimit` 模块，作为第二道防线。

### 7.5 Nginx 配置（`deploy/nginx/nginx.conf`，与 Caddy 二选一）

- **真实 IP 还原（自动同步版）**：`nginx.conf` 的 `http {}` 内 `include /etc/nginx/conf.d/cloudflare-realip.conf;`，该文件由 `deploy/scripts/update-cloudflare-ip.sh` 生成，包含全部 CF v4/v6 CIDR 的 `set_real_ip_from` + `real_ip_header CF-Connecting-IP;` + `real_ip_recursive on;`kbeezie.com+1。**禁止在 nginx.conf 中硬编码任何 CF CIDR**。
- 同步脚本必须先 `nginx -t` 校验通过再 `systemctl reload nginx`，失败时保留旧文件并告警。
- `limit_req_zone` 按 IP 限流，burst + nodelay。
- `client_max_body_size 0`（拒绝 body）、`client_header_buffer_size 1k`、`large_client_header_buffers 2 1k`。
- HTTP/2、TLS1.2/1.3、HSTS、`X-Content-Type-Options: nosniff` 等安全头。
- upstream keepalive，防连接耗尽。

------

## 8. SEO 与内容模块（`docs/seo.md` + 前端内联）

### 8.1 技术 SEO 清单

- `title / description / keywords` 按语言分版本，主关键词：IP查询、IPv6 检测、我的 IP、What is my IP。
- `canonical`、`hreflang`（zh-CN / en）。
- Schema.org：`WebSite` + `SoftwareApplication`（含 `applicationCategory: UtilitiesApplication`）。
- OpenGraph + Twitter Card。
- `sitemap.xml`、`robots.txt`（允许全站，disallow `/health` `/metrics`）。
- Core Web Vitals：LCP < 1.2s，CLS < 0.1，INP < 200ms。

### 8.2 内容矩阵

| 类型     | 页面                                                         | 目标       |
| -------- | ------------------------------------------------------------ | ---------- |
| 工具页   | `/`（IP 查询 + IPv6 检测）                                   | 主流量入口 |
| 知识页   | `/docs/what-is-ipv4`、`/docs/what-is-ipv6`、`/docs/ipv6-test-guide` 等 | 长尾 SEO   |
| 工具入口 | `/tools/subnet-calc`（预留）                                 | 横向扩展   |

------

## 9. 文档模块（`docs/`）

必须输出并保持与代码同步：

| 文件              | 内容要点                                                     |
| ----------------- | ------------------------------------------------------------ |
| `product.md`      | 产品定位、用户画像、使用场景、竞品分析（ip.sb / ipinfo.io / ipify） |
| `architecture.md` | 架构图、数据流、决策记录（ADR）                              |
| `development.md`  | 本地开发、构建、测试、贡献规范                               |
| `deployment.md`   | systemd（默认）+ Docker（可选）双轨部署、Caddy/Nginx 二选一、DNS 配置 |
| `operation.md`    | 日志、监控、故障 Runbook、备份、应急流程（被 DDoS/CC 时：CF Under Attack 模式 → 临时收紧限流 → 黑名单注入 → 通报）日志、监控、故障 Runbook、备份、应急流程（DDoS/CC）、**Cloudflare CIDR 自动同步运维（脚本位置、cron 周期、失败回滚、变更告警、源站防火墙只放行 CF CIDR）** |
| `security.md`     | 四层防御矩阵、IP 信任链、限流策略、应急封禁流程、隐私合规声明 |
| `seo.md`          | 关键词、Schema、内容日历、外链策略                           |
| `release.md`      | 版本规范、CI/CD、灰度发布策略（蓝绿/金丝雀：DNS 或 CF Load Balancer 切流，新版先 10% 观察 30 分钟）、回滚机制 |
| `privacy.md`      | 隐私合规说明。明确日志 IP 脱敏、无 Cookie、无 PII 收集、返回数据仅用于即时展示不持久化。 |

## 10. Agent 执行流程（交付物驱动 + 检查点）

```mermaid
flowchart TD
    P0["Phase 0<br/>变量填充"] --> P1
    P1["Phase 1<br/>架构设计"] --> CK1{Checkpoint 1}
    CK1 -->|通过| P2
    CK1 -->|回退| P1
    P2["Phase 2<br/>目录与契约"] --> CK2{Checkpoint 2}
    CK2 -->|通过| P3
    CK2 -->|回退| P2
    P3["Phase 3<br/>后端实现"] --> CK3{"Checkpoint 3<br/>go build && go test 全绿<br/>单测覆盖: IP解析/限流/超时<br/>curl localhost:8080/ 返回纯 IP"}
    CK3 -->|通过| P4
    CK3 -->|回退| P3
    P4["Phase 4<br/>前端实现"] --> CK4{"Checkpoint 4<br/>Lighthouse 各项达标<br/>Playwright E2E 测试全绿<br/>含跨浏览器及 IPv6 mock"}
    CK4 -->|通过| P5
    CK4 -->|回退| P4
    P5["Phase 5<br/>部署产物"] --> CK5{"Checkpoint 5<br/>systemctl start<br/>docker build"}
    CK5 -->|通过| P6
    CK5 -->|回退| P5
    P6["Phase 6<br/>部署文档"] --> P7["Phase 7<br/>SEO/文档"] --> P8["Phase 8<br/>生产 Review"]
```

### Phase 0：变量填充

- 输出 `VARIABLES.md`：列出所有 `{{VAR}}` 及其默认值，等待用户确认。
- 占位符示例：`{{ORG}}`、`{{CF_API_TOKEN}}`、`{{ORIGIN_IPV4}}`、`{{ORIGIN_IPV6}}`、`{{RATE_PER_IP}}` 等。
- Agent 必须在项目根目录生成 `.env.example`（或 `VARIABLES.md`），列出所有部署相关的变量及默认值。后续所有配置文件（如 `config.yaml`、`Caddyfile`、`systemd service`）中，凡是涉及域名、源站IP、API Token、限流参数的值，必须使用对应的 `{{VAR}}` 占位符（如 `{{ORIGIN_IPV4}}`），由部署脚本或用户手动替换，禁止硬编码。

### Phase 1：架构设计

- 交付物：`docs/architecture.md`（含图）、`docs/adr/0001-*.md` 起。
- Checkpoint：用户确认架构与 DNS 拓扑。

### Phase 2：目录与接口契约

- 交付物：仓库目录树、`openapi.yaml`（极简）、接口契约表。
- Checkpoint：契约评审通过。

### Phase 3：后端实现

- 交付物：`backend/` 完整代码、`go.mod`、单元测试（覆盖率 ≥ 70%）、`Makefile`。
- Checkpoint：`go build ./... && go test ./...` 全绿；`curl localhost:8080/` 返回纯 IP。

### Phase 4：前端实现

- 交付物：`frontend/` 完整代码、`_headers`、`_redirects`、`i18n.js`。
- Checkpoint：Lighthouse 达标；IPv6 不可达场景提示正确；Safari 真机验证。

### Phase 5：部署产物

- 交付物：`deploy/systemd/*.service`、`deploy/caddy/Caddyfile`、`deploy/nginx/nginx.conf`、`docker/Dockerfile`、`docker-compose.yml`、`scripts/install-systemd.sh`。

- Checkpoint：本地 `systemctl start` + `docker build` + `docker run` 均通过；`systemd-analyze security ip-lookup` 评分 ≤ 3.0。

- deploy/scripts/update-cloudflare-ip.sh`：一键拉取 CF v4/v6 列表，生成 Nginx + Caddy + Go 三份配置片段，校验后原子替换并触发 reload / fsnotify 热加载；失败保留旧配置并写 ERROR。

- `deploy/scripts/install-cf-sync-cron.sh`：安装 cron 条目（`0 3 * * *` 全量 + `0 */6 * * *` 增量校验）或 systemd timer 单元。

- `deploy/nftables/cloudflare-only.nft`：源站防火墙规则，仅放行 CF CIDR 访问 80/443，其余 DROP，作为信任链失效时的最后防线cloudflare.com。

- 交付物增加：

  ```
  scripts/verify.sh
  ```

   门禁脚本，用于自动化验收。脚本必须包含以下检查步骤，任一步骤失败则退出码为 1：

  1. `go test ./...`（后端单测必须全绿）
  2. `golangci-lint run`（代码静态检查）
  3. `docker build -t ip-lookup:test .`（Docker 镜像必须能成功构建）
  4. 启动 Docker 容器，`curl localhost:8080/health` 返回 200。
  5. `systemd-analyze security deploy/systemd/ip-lookup.service` 且评分 ≤ 3.0。
  6. 停止容器，清理环境。

### Phase 6：部署文档

- 交付物：`docs/deployment.md`（systemd 默认章节在前，Docker 在后标"可选"）。

### Phase 7：SEO 与文档

- 交付物：`docs/seo.md`、`sitemap.xml`、`robots.txt`、Schema 内联、全量文档树。

### Phase 8：生产级 Review

- 交付物：`docs/release.md` + `REVIEW.md`（对照第 11 节逐项打勾）。
- Checkpoint：全部验收项 ✅。
- 交付物增加：`.github/workflows/ci.yml` (GitHub Actions 配置)。
- 触发条件：`on: [pull_request]`。
- Job 步骤：Checkout 代码 -> 安装 Go -> 安装依赖 -> 执行 `bash scripts/verify.sh`。如果脚本失败，CI 标红阻断合并。

------

## 11. 验收标准（门禁清单）

### 11.1 功能与体验

- 页面打开立即反馈，加载态、成功态、失败态、超时态、节流态五态齐全
- IPv6 不可达时正确提示，不报 JS 错误
- 连续点击不产生重复真实请求（60s 节流）
- 移动端 Safari / Android 浏览器流畅
- zh/en 自动切换，无切换按钮
- Lighthouse Accessibility 达标，aria 属性与对比度验证通过

### 11.2 后端与安全

- `GET /` 返回纯 IP，`Content-Type: text/plain`，无尾换行
- `/health` `/readyz` 可用
- 真实 IP 信任链：伪造 `X-Forwarded-For` 不可绕过限流
- 限流触发返回 `429` + `Retry-After`
- 优雅退出：`SIGTERM` 后 15s 内退出 0
- `systemd-analyze security` 评分 ≤ 3.0
- 日志中 IP 已脱敏，无明文记录完整 IP
- `/metrics` 端点可用并暴露标准 Prometheus 指标
- 临时修改 `cf-cidrs.txt` 后，Go 进程在不重启的情况下 5 秒内生效新的可信 CIDR
- 断网模拟拉取失败时，现有 `cloudflare-realip.conf` 不被破坏，`nginx -t` 仍通过
- 源站防火墙启用后，非 CF IP 直连 80/443 被拒绝

### 11.3 部署与可运维

- `systemctl start ip-lookup` 可启动，`systemctl status` 正常
- `docker build` + `docker run` 可启动（可选路径）
- Caddy 或 Nginx 二选一配置完整、HTTPS 自动签发
- 日志结构化 + 轮转
- 监控指标端点可用

### 11.4 SEO 与文档

- Lighthouse 各项 ≥ 95
- Schema.org、OG、sitemap、robots 齐全
- `docs/` 八份文档齐全且与代码一致
- 仓库可直接 push 到 GitHub 并 CI 通过

------

## 12. Agent 行为约束

1. **不臆测**：遇到未定义变量或冲突需求，停止并提问，禁止自行编造。
2. **不省略**：所有 config / unit / Dockerfile / Caddyfile / nginx.conf 须完整输出，禁止"..."省略。
3. **不跳阶段**：未通过 Checkpoint 不得进入下一 Phase。
4. **不静默改技术栈**：替换依赖须在 ADR 中说明理由。
5. **变更可追溯**：每个 Phase 产出 `CHANGELOG.md` 条目。

------
