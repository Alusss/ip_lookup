(function () {
  'use strict';

  // ===================================================
  // 站点可配置信息 / Site Configurable Data
  // 修改此处即可更新全局占位信息，后续替换成你自己的数据
  // ===================================================
  var SITE_CONFIG = {
    email: 'admin@example.com',           // 联系邮箱（中英文界面均显示）
    icp: '湘ICP备18010752号-2',            // ICP 备案号（仅中文界面显示）
    gongan: '湘公网安备43010402002914号', // 公安备案号（仅中文界面显示）
  };

  var I18N = {
    zh: {
      title: 'IP 查询 - 查看你的 IP 地址 & IPv6 连通性检测',
      desc: '免费的 IP 地址查询工具，快速查看你的公网 IPv4/IPv6 地址并检测 IPv6 连通性。',
      og_title: 'IP 查询 - 在线 IP 地址查询工具',
      og_desc: '快速查询你的公网 IP 地址并检测 IPv6 连通性。',
      site_name: 'IP 查询',
      tagline: '快速查询您的公网 IP 地址',
      ipv4_label: 'IPv4 地址',
      your_ip_label: '你的 IP 地址',
      loading: '正在获取你的 IP 地址...',
      copy: '复制',
      copy_success: '已复制',
      refresh: '刷新',
      refreshing: '刷新中',
      success: '获取成功',
      error: '获取 IP 地址失败，请稍后重试',
      timeout: '请求超时，请检查网络',
      throttled: '已是最新结果，请稍后刷新',
      ipv6_title: 'IPv6 地址',
      ipv6_subtitle: '自动检测中',
      ipv6_checking: '正在检测 IPv6 连接...',
      ipv6_success: '你的 IPv6 地址：{ip}',
      ipv6_fail: '你的网络未启用 Internet IPv6',
      privacy: '隐私政策',
      learn_ipv6: '了解 IPv6',
      email_label: '📧 邮箱',
    },
    en: {
      title: 'IP Lookup - Check Your IP Address & IPv6 Connectivity',
      desc: 'Free IP address lookup tool. Check your public IPv4/IPv6 address and test IPv6 connectivity instantly.',
      og_title: 'IP Lookup - Online IP Address Checker',
      og_desc: 'Check your public IP address and test IPv6 connectivity.',
      site_name: 'IP Lookup',
      tagline: 'Check your public IP address instantly',
      ipv4_label: 'IPv4 Address',
      your_ip_label: 'Your IP Address',
      loading: 'Getting your IP address...',
      copy: 'Copy',
      copy_success: 'Copied',
      refresh: 'Refresh',
      refreshing: 'Refreshing...',
      success: 'Success',
      error: 'Failed to get IP address. Please try again later.',
      timeout: 'Request timed out. Please check your network.',
      throttled: 'Already up to date. Please try later.',
      ipv6_title: 'IPv6 Address',
      ipv6_subtitle: 'Automatically detected',
      ipv6_checking: 'Testing IPv6 connection...',
      ipv6_success: 'Your IPv6 address: {ip}',
      ipv6_fail: 'Your network does not support IPv6 connectivity',
      privacy: 'Privacy Policy',
      learn_ipv6: 'Learn about IPv6',
      email_label: '📧 Email',
    },
  };

  function getLang() {
    var lang = navigator.language || navigator.userLanguage || 'en';
    return lang.startsWith('zh') ? 'zh' : 'en';
  }

  function t(key) {
    var lang = getLang();
    return I18N[lang][key] || I18N['en'][key] || key;
  }

  function applyI18n() {
    var lang = getLang();
    var isZh = lang === 'zh';

    document.documentElement.lang = isZh ? 'zh-CN' : 'en';

    document.querySelectorAll('[data-i18n]').forEach(function (el) {
      var key = el.dataset.i18n;
      var text = t(key);
      if (el.tagName === 'META') {
        el.setAttribute('content', text);
      } else if (el.tagName === 'TITLE') {
        el.textContent = text;
      } else {
        el.textContent = text;
      }
    });

    document.querySelectorAll('[data-i18n-attrs]').forEach(function (el) {
      el.dataset.i18nAttrs.split(';').forEach(function (attr) {
        var parts = attr.split(':');
        el.setAttribute(parts[0], t(parts[1]));
      });
    });

    var emailAddr = document.getElementById('email-addr');
    if (emailAddr) emailAddr.textContent = SITE_CONFIG.email;

    var filingDiv = document.getElementById('footer-filing');
    if (filingDiv) {
      if (isZh && (SITE_CONFIG.icp || SITE_CONFIG.gongan)) {
        filingDiv.classList.remove('hidden');
        var parts = [];
        if (SITE_CONFIG.icp) parts.push('<span>' + SITE_CONFIG.icp + '</span>');
        if (SITE_CONFIG.gongan) parts.push('<span><img src="img/gonganbeian.png" alt="" style="width:16px;height:16px;vertical-align:middle;margin-right:3px">' + SITE_CONFIG.gongan + '</span>');
        filingDiv.innerHTML = parts.join(' | ');
      } else {
        filingDiv.classList.add('hidden');
      }
    }
  }

  window.getLang = getLang;
  window.t = t;

  document.addEventListener('DOMContentLoaded', applyI18n);
})();
