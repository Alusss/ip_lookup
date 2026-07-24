# SEO 策略与配置

## 技术 SEO

### 标题与描述

| 语言 | Title | Description |
|------|-------|-------------|
| zh | IP 查询 - 查看你的 IP 地址 & IPv6 连通性检测 | 免费的 IP 地址查询工具，快速查看你的公网 IPv4/IPv6 地址并检测 IPv6 连通性。 |
| en | IP Lookup - Check Your IP Address & IPv6 Connectivity | Free IP address lookup tool. Check your public IPv4/IPv6 address and test IPv6 connectivity instantly. |

### 主要关键词

- IP 查询 / IP Lookup / IP Address Lookup
- 我的 IP / What is my IP / My IP Address
- IPv6 检测 / IPv6 Test / IPv6 Connectivity / IPv6 测试
- 公网 IP / Public IP / IPv6 地址查询

### Schema.org

```json
{
  "@context": "https://schema.org",
  "@type": "WebSite",
  "name": "IP Lookup",
  "url": "https://ip.iohow.com/",
  "description": "Check your public IP address and test IPv6 connectivity instantly.",
  "applicationCategory": "UtilitiesApplication",
  "operatingSystem": "All"
}
```

### 页面标签

- `<link rel="canonical" href="https://ip.iohow.com/">`
- `<link rel="alternate" hreflang="zh-CN" href="https://ip.iohow.com/">`
- `<link rel="alternate" hreflang="en" href="https://ip.iohow.com/en/">`
- OpenGraph + Twitter Card 标签

### 机器人文件

- `frontend/robots.txt`（Allow 全站，Disallow `/health` `/readyz` `/metrics` `/ad-config`）
- `sitemap.xml`（含 alternate hreflang）

### 性能目标（Core Web Vitals）

- LCP < 1.2s
- CLS < 0.1
- INP < 200ms
- Lighthouse Performance ≥ 95

---

## 内容矩阵

| 页面 | URL | 目标关键词 | 状态 | 说明 |
|------|-----|-----------|------|------|
| IP 查询 | `/` | IP查询, 我的IP, What is my IP | ✅ | 主流量入口 |
| IPv6 介绍 | `/docs/what-is-ipv6` | What is IPv6, IPv6 地址, IPv6 是什么 | ✅ | 已上线 |
| IPv6 检测指南 | `/docs/ipv6-test-guide` | IPv6 test, IPv6 检测, 如何检测 IPv6 | ✅ | 已上线 |
| IPv4 介绍 | `/docs/what-is-ipv4` | What is IPv4, IPv4 地址 | 📋 预留 | 下一版本 |
| 子网计算器 | `/tools/subnet-calc` | Subnet calculator, 子网计算 | 📋 预留 | 下一版本 |

---

## 外链策略

- 各大开源项目 README 引入
- 开发者社区（V2EX、Reddit r/ipv6、Hacker News）
- 免费工具导航站收录
- 知识页之间交叉链接（工具页 ↔ 知识页）

---

## 站点文件

- `/sitemap.xml`
- `/robots.txt`
