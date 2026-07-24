(function () {
  'use strict';

  // Site-wide i18n policy: ONLY Simplified Chinese browsers (zh-CN / zh-Hans /
  // zh-SG / bare zh) get the Chinese UI; every other locale (including zh-TW,
  // zh-HK, zh-Hant) gets English.
  function isZhLang(l) {
    l = (l || '').toLowerCase();
    return l === 'zh' || l.indexOf('zh-cn') === 0 || l.indexOf('zh-hans') === 0 || l.indexOf('zh-sg') === 0;
  }

  // Kill English flash for zh users: hide page until i18n applies.
  // Runs synchronously in <head> before first paint. Safety timeout
  // guarantees reveal even if DOMContentLoaded is delayed.
  if (isZhLang(navigator.language || '')) {
    document.documentElement.style.visibility = 'hidden';
    setTimeout(function () { document.documentElement.style.visibility = ''; }, 400);
  }

  // ===================================================
  // 站点可配置信息 / Site Configurable Data
  // ===================================================
  var SITE_CONFIG = {
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
      loading: '正在获取你的 IP 地址...',
      loading_short: '正在获取...',
      copy: '复制',
      copy_success: '已复制',
      refresh: '刷新',
      refreshing: '刷新中',
      success: '获取成功',
      error: '获取 IP 地址失败，请稍后重试',
      timeout: '请求超时，请检查网络',
      throttled: '已是最新结果，请稍后刷新',
      ipv6_label: 'IPv6 地址',
      ipv6_testing: '正在检测...',
      ipv6_not_supported: '未检测到 IPv6 连接',
      privacy: '隐私政策',
      email_label: '邮箱',
      copyright: 'IOHOW 版权所有 © {year}',
      learn_ipv6: '了解 IPv6',
      kb_ipv6: '什么是 IPv6？',
      kb_ipv6_test: 'IPv6 检测指南',
      kb_ip_addr: '什么是 IP 地址？',
      kb_pub_priv: '公网 IP 与内网 IP',
      kb_more: '了解更多 →',
      privacy_title: '隐私政策 - IP 查询',
      what_is_ipv6_title: '什么是 IPv6？简单指南 - IP 查询',
      ipv6_test_guide_title: 'IPv6 检测指南：如何检查 IPv6 连通性 - IP 查询',
      what_is_ip_address_title: '什么是 IP 地址？ - IP 查询',
      public_vs_private_ip_title: '公网 IP 与内网 IP 的区别 - IP 查询',
      docs_index_title: '网络知识库 - IP 查询',
      back_home: '返回 IP 查询',
      docs_intro: '关于 IP 地址、IPv6 与网络连通性的实用知识文章。',
      docs_cta: '立即检测你的 IP 与 IPv6 连通性 ->',
    },
    en: {
      title: 'IP Lookup - Check Your IP Address & IPv6 Connectivity',
      desc: 'Free IP address lookup tool. Check your public IPv4/IPv6 address and test IPv6 connectivity instantly.',
      og_title: 'IP Lookup - Online IP Address Checker',
      og_desc: 'Check your public IP address and test IPv6 connectivity.',
      site_name: 'IP Lookup',
      tagline: 'Instant Public IP Lookup',
      ipv4_label: 'IPv4 Address',
      loading: 'Getting your IP address...',
      loading_short: 'Loading...',
      copy: 'Copy',
      copy_success: 'Copied',
      refresh: 'Refresh',
      refreshing: 'Refreshing...',
      success: 'Success',
      error: 'Failed to get IP address. Please try again later.',
      timeout: 'Request timed out. Please check your network.',
      throttled: 'Already up to date. Please try later.',
      ipv6_label: 'IPv6 Address',
      ipv6_testing: 'Testing...',
      ipv6_not_supported: 'No IPv6 connectivity detected',
      privacy: 'Privacy Policy',
      email_label: 'Email',
      copyright: '© {year} IOHOW. All rights reserved.',
      learn_ipv6: 'Learn about IPv6',
      kb_ipv6: 'What is IPv6?',
      kb_ipv6_test: 'IPv6 Test Guide',
      kb_ip_addr: 'What is an IP Address?',
      kb_pub_priv: 'Public vs Private IP',
      kb_more: 'Learn more →',
      privacy_title: 'Privacy Policy - IP Lookup',
      what_is_ipv6_title: 'What is IPv6? A Simple Guide - IP Lookup',
      ipv6_test_guide_title: 'IPv6 Test Guide: How to Check IPv6 Connectivity - IP Lookup',
      what_is_ip_address_title: 'What is an IP Address? - IP Lookup',
      public_vs_private_ip_title: 'Public vs Private IP - IP Lookup',
      docs_index_title: 'Knowledge Base - IP Lookup',
      back_home: 'Back to IP Lookup',
      docs_intro: 'Practical articles about IP addresses, IPv6, and network connectivity.',
      docs_cta: 'Test your IP and IPv6 connectivity now ->',
    },
  };

  function getLang() {
    var lang = navigator.language || navigator.userLanguage || 'en';
    return isZhLang(lang) ? 'zh' : 'en';
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
        document.title = text;
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

    var ct = document.getElementById('copyright-text');
    if (ct) ct.textContent = t('copyright').replace('{year}', new Date().getFullYear());

    var filingDiv = document.getElementById('footer-filing');
    if (filingDiv) {
      filingDiv.textContent = '';
      if (isZh && (SITE_CONFIG.icp || SITE_CONFIG.gongan)) {
        filingDiv.classList.remove('hidden');
        if (SITE_CONFIG.gongan) {
          var sG = document.createElement('span');
          sG.className = 'gongan-badge';
          var img = document.createElement('img');
          img.src = 'img/gonganbeian.png';
          img.alt = '';
          sG.appendChild(img);
          sG.appendChild(document.createTextNode(SITE_CONFIG.gongan));
          filingDiv.appendChild(sG);
        }
        if (SITE_CONFIG.icp) {
          if (filingDiv.childNodes.length) filingDiv.appendChild(document.createTextNode(' | '));
          var sI = document.createElement('span');
          sI.textContent = SITE_CONFIG.icp;
          filingDiv.appendChild(sI);
        }
      } else {
        filingDiv.classList.add('hidden');
      }
    }

    document.documentElement.style.visibility = '';
  }

  window.getLang = getLang;
  window.t = t;

  document.addEventListener('DOMContentLoaded', applyI18n);
})();
