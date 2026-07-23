# Cloudflare Pages 部署

将 `frontend/` 静态页面通过 Cloudflare Pages 自动部署，同时保障后端代码不对外暴露。

---

## 架构概览

```
用户浏览器 ──► Cloudflare CDN ──► Cloudflare Pages (frontend/)
                    │
                    ├──► ip4.iohow.com (源站 VPS, Go API)
                    └──► ip6.iohow.com (源站 VPS, Go API)
```

- 静态页面（HTML/JS/CSS/图片）托管在 Cloudflare Pages
- IP 查询 API 仍由 VPS 上的 Go 后端处理，前端通过 `fetch()` 调用 `ip4.iohow.com` / `ip6.iohow.com`

---

## 安全边界

Cloudflare Pages **只发布 publish directory 内的文件**。本仓库其他目录和文件：

| 目录/文件 | 是否暴露 | 原因 |
|---|---|---|
| `frontend/` | ✅ 是 | 配置为 publish directory |
| `_headers`, `_redirects` | ✅ 是 | 构建时复制到 publish directory |
| `robots.txt` | ✅ 是 | 构建时复制到 publish directory |
| `backend/` | ❌ 否 | 不在 publish directory 内 |
| `api/` | ❌ 否 | 同上 |
| `deploy/` | ❌ 否 | 同上 |
| `docker/` | ❌ 否 | 同上 |
| `scripts/` | ❌ 否 | 同上 |
| `docs/` | ❌ 否 | 同上 |
| `.github/` | ❌ 否 | 同上 |
| `.env.example`, `Makefile` 等 | ❌ 否 | 同上 |

这是 Cloudflare Pages 的内置隔离机制，不需要额外配置。

---

## 第一步：在 Cloudflare Dashboard 连接仓库

### 1.1 登录 Cloudflare Dashboard

打开 https://dash.cloudflare.com/，进入 **Workers & Pages**。

### 1.2 创建 Pages 项目

1. 点击 **Create application** → **Pages** → **Connect to Git**
2. 选择 GitHub（或其他 Git provider）
3. 授权 Cloudflare 访问你的仓库
4. 选择 `ip-lookup` 仓库
5. 点击 **Begin setup**

### 1.3 配置构建设置

| 配置项 | 值 |
|---|---|
| **Project name** | `ip-lookup` |
| **Framework preset** | **None** |
| **Build command** | `cp _headers _redirects robots.txt frontend/` |
| **Build output directory** | `frontend` |
| **Root directory** | *留空* |
| **Branch** | `main` |

> **Build command 说明**：将仓库根目录的 `_headers`、`_redirects`、`robots.txt` 复制到 `frontend/` 目录下，使其进入发布范围。

### 1.4 配置环境变量（可选）

不需要设置环境变量。前端页面通过 JS 直接调用后端 API，不涉及构建时变量。

### 1.5 点击 **Save and Deploy**

首次部署会自动触发，后续每次推送到 `main` 分支会自动重新部署。

---

## 第二步：配置自定义域名

### 2.1 添加域名

1. 在 Pages 项目页面，进入 **Custom domains** 选项卡
2. 点击 **Set up a custom domain**
3. 输入 `ip.iohow.com`
4. 确保 Cloudflare 中该域名的 DNS 记录已通过 Cloudflare 托管

### 2.2 DNS 记录确认

确保 Cloudflare DNS 中已有以下记录（通常由第一步自动添加）：

| 类型 | 名称 | 目标 | Proxy |
|---|---|---|---|
| CNAME | `ip` | `ip-lookup.pages.dev` | Proxied (橙色云) |

### 2.3 SSL/TLS 加密

Cloudflare Pages 自动管理 SSL 证书，无需手动配置。

---

## 第三步：自动部署验证

### 推送触发

```bash
git push origin main
```

Cloudflare Pages 会自动检测到推送并触发部署，可在 **Deployments** 选项卡实时查看构建日志。

### 手动重新部署

在 Cloudflare Dashboard → Pages → `ip-lookup` → **Deployments** → **Trigger deployment**

### 预览部署（Preview Deployments）

向非 `main` 分支推送时，Cloudflare Pages 会自动生成一个预览 URL：

```
https://<branch-name>.ip-lookup.pages.dev
```

可用于上线前验证。

---

## 常见问题

### Q: 构建失败怎么办？

检查 Build command 是否正确，以及 `frontend/` 目录是否存在。可在 Cloudflare Dashboard 查看构建日志定位错误。

### Q: 页面部署后 JS/CSS 不生效？

确认 `_headers` 文件已正确复制到发布目录。`_headers` 中的路径规则以站点根目录为基准（`/js/*.js` 而非 `/frontend/js/*.js`）。

### Q: API 请求被 CSP 拦截？

检查 `_headers` 中的 `Content-Security-Policy` 的 `connect-src` 是否包含了你的 API 域名：

```http
Content-Security-Policy: default-src 'self'; connect-src 'self' https://ip4.iohow.com https://ip6.iohow.com; ...
```

### Q: 如何回滚到上一版本？

在 **Deployments** 页面找到目标部署，点击右侧的 `...` → **Rollback to this deployment**。

---

## DNS 总览

| 域名 | 用途 | 托管位置 | Proxy |
|---|---|---|---|
| `ip.iohow.com` | 静态页面（Pages） | Cloudflare CDN | Proxied |
| `ip4.iohow.com` | IPv4 API（Go 后端） | 源站 VPS | Proxied |
| `ip6.iohow.com` | IPv6 API（Go 后端） | 源站 VPS | Proxied |

---

> 参见：[部署总览](deployment.md)、[安全策略](security.md)
