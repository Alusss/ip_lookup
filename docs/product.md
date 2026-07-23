# 产品文档

## 产品定位

极简、零广告（对终端用户）、毫秒级、SEO 友好的公网 IP 查询与 IPv6 连通性检测工具。

## 目标用户

| 画像 | 场景 | 需求 |
|------|------|------|
| 普通用户 | 想知道自己的公网 IP | 打开即看，一次获得 IPv4 + IPv6 |
| 开发者 | 调试网络/CDN，集成 API | 纯文本 API + JSON API（含 GeoIP） |
| 网络运维 | 排查 IPv6 连通性 | 自动检测 IPv6，无需手动操作 |
| SEO 流量 | 搜索"我的 IP"/"IPv6 是什么" | 工具页 + 知识页双轨覆盖 |

## 核心功能

1. **双栈 IP 查询**：自动并行检测并展示 IPv4 和 IPv6 地址
2. **一键复制**：点击复制 IP 地址到剪贴板
3. **IPv6 自动检测**：页面加载时自动检测，无需手动点击
4. **JSON API**：携带 `Accept: application/json` 获取结构化数据（含城市、国家、ISP）
5. **GeoIP 地理位置**（可选）：启用后 JSON API 返回客户端 IP 的地理位置信息
6. **多语言**：自动中英文切换
7. **多端适配**：桌面和移动端均优

## API 使用

```bash
# 纯 IP（前端用）
curl -H "X-Client: web" https://ip4.iohow.com/
# → 203.0.113.42

# 带广告（直接访问）
curl https://ip4.iohow.com/
# → 广告文案 (URL)
# → 203.0.113.42

# JSON（含 GeoIP）
curl -H "Accept: application/json" https://ip4.iohow.com/
# → {"ip":"203.0.113.42","version":"IPv4","city":"...","country":"...","isp":"..."}
```

## 竞品分析

| 产品 | 优势 | 劣势 |
|------|------|------|
| ip.sb | 极简，速度快 | 无 IPv6 检测 |
| ipinfo.io | 信息丰富，API 完善 | 对普通用户过于复杂，有速率限制 |
| ipify | 轻量 API | 无前端页面 |
| whatismyip.com | 品牌强 | 广告多，速度慢 |

**差异化**：IPv4 + IPv6 双栈自动检测 + JSON API（含 GeoIP）+ 隐私合规。

## 商业模式

- API 广告（curl/浏览器直接访问 API 时展示一行文字广告，`api_ad_enabled`）
- Web 广告（前端页面顶栏横幅，`web_ad_enabled`，用户可关闭）
- 前端用户核心功能免费、无需注册、无侵入式广告
