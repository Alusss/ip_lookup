# 部署文档

## 部署方式选择

| 维度 | systemd 二进制（默认） | Docker（可选） |
|------|------------------------|----------------|
| 适用场景 | 单 VPS 长期运行、最小开销 | 多实例/容器编排/CI 产物分发 |
| 资源占用 | 极低（~5MB 二进制） | 略高（含基础镜像） |
| 可观测 | journald + systemctl status | docker logs + healthcheck |
| 端口绑定 | CAP_NET_BIND_SERVICE 或 `setcap` | 端口映射 |
| 推荐度 | ★★★★★（首选） | ★★★（需要时启用） |

---

## 默认部署：systemd 二进制

### 前置要求

- Go 1.23+（构建用）
- systemd（运行环境）
- 非 root 用户执行构建

### 构建

```bash
cd backend
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ip-lookup .
```

### 安装

```bash
sudo bash scripts/install-systemd.sh ./backend/ip-lookup
```

该脚本自动执行：
1. 创建 `iplookup` 用户和组
2. 创建数据目录 `/var/lib/ip-lookup`（含 GeoIP 数据库）和日志目录 `/var/log/ip-lookup`
3. 安装二进制到 `/usr/local/bin/ip-lookup`
4. 安装默认配置到 `/etc/ip-lookup/config.yaml`（如已存在则保留）
5. 设置 `cap_net_bind_service` 兜底
6. 安装并启用 systemd 服务（双栈监听 8080/8081）

### 配置

编辑 `/etc/ip-lookup/config.yaml` 或创建 `/etc/ip-lookup/env` 环境变量文件：

```bash
# /etc/ip-lookup/env
API_AD_ENABLED=true
API_AD_TEXT_ZH=推荐使用可靠的VPN服务保护您的隐私
API_AD_URL_ZH=https://example.com/zh/vpn
API_AD_TEXT_EN=Recommended VPN service for privacy
API_AD_URL_EN=https://example.com/en/vpn
WEB_AD_ENABLED=true
WEB_AD_TEXT_ZH=推荐使用可靠的VPN服务保护您的隐私
WEB_AD_URL_ZH=https://example.com/zh/vpn
WEB_AD_TEXT_EN=Recommended VPN service for privacy
WEB_AD_URL_EN=https://example.com/en/vpn
GEOIP_ENABLED=false
```

### 内存限制

systemd 单元已配置 `MemoryMax=256M` + `MemoryHigh=192M`，防止极端场景下 OOM。如需调整：

```bash
sudo systemctl edit ip-lookup.service
# 添加：
# [Service]
# MemoryMax=512M
# MemoryHigh=384M
sudo systemctl daemon-reload && sudo systemctl restart ip-lookup
```

### 自监控告警配置

编辑 `/etc/ip-lookup/config.yaml` 启用自监控：

```yaml
monitoring:
  enabled: true                    # 开启自监控
  check_interval: 60s              # 每 60 秒检查一次
  alert_cooldown: 10m              # 同一指标 10 分钟内不重复告警
  alert_webhook_url: "https://your-webhook-gateway/alert"  # 推送地址
  alert_webhook_type: "generic"    # generic 或 dingtalk
  error_rate_threshold: 0.05       # 5% 错误率告警
  p99_latency_threshold_ms: 2000   # P99 > 2s 告警
  rate_limit_hit_rate_threshold: 0.10  # 10% 限流率告警
```

也可通过环境变量覆盖（适合 `/etc/ip-lookup/env`）：

```bash
MONITORING_ENABLED=true
MONITORING_WEBHOOK_URL=https://your-webhook-gateway/alert
MONITORING_WEBHOOK_TYPE=generic
```

### 验证部署

```bash
# 健康检查
curl localhost:8080/health    # → OK
curl localhost:8080/readyz    # → OK

# IP 查询（纯文本）
curl http://127.0.0.1:8080/   # → <your IPv4>

# JSON API
curl -H "Accept: application/json" http://127.0.0.1:8080/   # → {"ip":"...","version":"IPv4",...}

# 广告配置
curl http://127.0.0.1:8080/ad-config   # → {"web":{"enabled":true,...}}

# 指标（独立端口，仅本地可达）
curl http://127.0.0.1:9090/metrics

# systemd 安全评分
systemd-analyze security ip-lookup    # 目标 ≤ 3.0
```

---

## 可选部署：Docker

```bash
docker build -t ip-lookup:latest -f docker/Dockerfile .
docker run -d --name ip-lookup \
  --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v /etc/ip-lookup/config.yaml:/etc/ip-lookup/config.yaml:ro \
  -v /var/log/ip-lookup:/var/log/ip-lookup \
  -v /var/lib/ip-lookup:/var/lib/ip-lookup \
  ip-lookup:latest
```

---

## 源站 Web 服务器

### Caddy（默认推荐）

`deploy/caddy/Caddyfile`：
- 自动 HTTPS（Cloudflare DNS-01 challenge）
- `ip4` → `bind 0.0.0.0` → `reverse_proxy 127.0.0.1:8080`
- `ip6` → `bind [::]` → `reverse_proxy [::1]:8081`

### Nginx（备选）

`deploy/nginx/nginx.conf`：

**SSL 证书**（Nginx 不自签，需手动获取）：
```bash
# 推荐 acme.sh + Cloudflare DNS-01 自动续期
curl https://get.acme.sh | sh
acme.sh --issue --dns dns_cf -d ip4.iohow.com -d ip6.iohow.com
acme.sh --install-cert -d ip4.iohow.com \
  --key-file /etc/nginx/ssl/ip4.iohow.com/privkey.pem \
  --fullchain-file /etc/nginx/ssl/ip4.iohow.com/fullchain.pem \
  --reloadcmd "systemctl reload nginx"
acme.sh --install-cert -d ip6.iohow.com \
  --key-file /etc/nginx/ssl/ip6.iohow.com/privkey.pem \
  --fullchain-file /etc/nginx/ssl/ip6.iohow.com/fullchain.pem \
  --reloadcmd "systemctl reload nginx"
```

**⚠️ 关键配置：真实 IP 传递**
Nginx 和 Go 后端在同一台机器上时，Go 看到的 `RemoteAddr` 为 `127.0.0.1`。Nginx 配置通过以下三个 `proxy_set_header` 将客户端信息转发给后端：

```nginx
proxy_set_header Host $host;                              # 保持原始 Host
proxy_set_header X-Forwarded-For $remote_addr;            # 原始连接真实 IP（最右跳）
proxy_set_header CF-Connecting-IP $http_cf_connecting_ip; # Cloudflare 原始客户端 IP
```

**后端必须将本地代理加入可信列表**（已在 `config.yaml` 中设置 `trusted_proxy_cidrs: "127.0.0.1/32,::1/128"`），否则 Go 不会信任转发过来的 IP 头。

**🩺 健康检查端点绕过限流**
监控系统（Prometheus Blackbox、systemd healthchecks）会持续请求 `/health` 和 `/readyz`。这两个 location 块显式声明了 `limit_req off`，避免被限流误杀导致误报警。

**🔒 安全头**
Nginx 层额外施加了 `Permissions-Policy: camera=(), microphone=(), geolocation=()`，限制浏览器 API 权限（与 Go 层的安全头互补，Go 不覆盖 Nginx 已发出的头）。

**🔁 Upstream 被动健康检查**
```nginx
upstream backend_v4 {
    server 127.0.0.1:8080 max_conns=256 max_fails=3 fail_timeout=10s;
}
```
Nginx 开源版无主动健康检查模块，通过 `max_fails=3 fail_timeout=10s` 实现被动容错：连续 3 次失败后将该后端标记为不可用 10 秒。配合 `keepalive 32` 复用连接。

**Nginx 与 Caddy 对比**：

| 维度 | Caddy | Nginx |
|------|-------|-------|
| TLS 证书 | 自动（Cloudflare DNS-01） | 手动，需额外部署 acme.sh |
| 真实 IP 传递 | 内置 `client_ip_headers` 指令 | 需手动配置 `proxy_set_header` |
| 主动健康检查 | 内置 `health_uri` | 仅被动检查（`max_fails`），开源版无主动检查 |
| 安全头 | `header` 指令 | `add_header` 指令，需逐一声明 |
| 配置复杂度 | 低 | 中 |

> 切换到 Nginx 后如需回退到 Caddy，只需：1. 安装 Caddy + cloudflare DNS 插件；2. 复制 `deploy/caddy/Caddyfile`；3. 修改 Go 配置 `trusted_proxy_cidrs` 为空（Caddy 通过 trusted_proxies 模块处理真实 IP 传递）。

---

## DNS 配置

| 域名 | 类型 | 值 | 说明 |
|------|------|----|------|
| ip.iohow.com | A + AAAA | Cloudflare CDN Proxied | 主站静态页面 |
| ip4.iohow.com | A | 源站 IPv4 | IPv4 API |
| ip6.iohow.com | AAAA | 源站 IPv6 | IPv6 API |

---

## Cloudflare CIDR 自动同步

```bash
# cron 方式
sudo bash deploy/scripts/install-cf-sync-cron.sh cron
# systemd timer 方式
sudo bash deploy/scripts/install-cf-sync-cron.sh timer
# 手动同步
sudo bash deploy/scripts/update-cloudflare-ip.sh
```

同步产物：
- Nginx: `/etc/nginx/conf.d/cloudflare-realip.conf`
- Caddy: `/etc/caddy/cloudflare-trusted.conf`
- Go: `/etc/ip-lookup/cf-cidrs.txt`（fsnotify 热加载）
- nftables: `/etc/nftables/cloudflare-cidr.nft`（仅 nftables 已安装时生成）

---

## GeoIP 数据库（可选）

### 首次部署

1. 注册免费 MaxMind 账号：https://www.maxmind.com/en/geolite2/signup
2. 生成 License Key
3. 运行更新脚本：
```bash
sudo MAXMIND_LICENSE_KEY=your_key bash deploy/scripts/update-geoip.sh
```

### 自动更新（cron）

```bash
# 每周日凌晨 4 点更新
echo "0 4 * * 0 root MAXMIND_LICENSE_KEY=your_key /etc/ip-lookup/update-geoip.sh" > /etc/cron.d/geoip-update
```

### GeoIP 配置

```yaml
# config.yaml
geoip_enabled: true
geoip_db_path: /var/lib/ip-lookup/GeoLite2-City.mmdb
geoip_asn_db_path: /var/lib/ip-lookup/GeoLite2-ASN.mmdb  # 可选，提供 ASN
```

数据库更新后 Go 进程自动通过 fsnotify 加载，无需重启；修改开关/路径也可通过配置热加载生效。

---

## 源站防火墙（可选）

```bash
cp deploy/nftables/cloudflare-only.nft /etc/nftables/
nft -f /etc/nftables/cloudflare-only.nft
```
