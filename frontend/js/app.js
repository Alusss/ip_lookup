(function () {
  'use strict';

  var IP4_API = 'https://ip4.iohow.com/';
  var IP6_API = 'https://ip6.iohow.com/';
  var FETCH_TIMEOUT_IP4 = 5000;
  var FETCH_TIMEOUT_IP6 = 8000;
  var THROTTLE_MS = 60000;
  var AD_SESSION_KEY = 'ad_closed';

  var ipDisplay = document.getElementById('ip-address');
  var statusMsg = document.getElementById('status-msg');
  var geoLine = document.getElementById('geo-line');
  var copyBtn = document.getElementById('copy-btn');
  var refreshBtn = document.getElementById('refresh-btn');
  var ipv4Card = document.getElementById('ipv4-card');

  var ipv6Address = document.getElementById('ipv6-address');
  var ipv6Status = document.getElementById('ipv6-status');
  var ipv6CopyBtn = document.getElementById('ipv6-copy-btn');
  var ipv6RefreshBtn = document.getElementById('ipv6-refresh-btn');
  var ipv6Card = document.getElementById('ipv6-card');

  var adBar = document.getElementById('ad-bar');
  var adLink = document.getElementById('ad-link');
  var closeAd = document.getElementById('close-ad');

  var copyBtnText = copyBtn && copyBtn.querySelector('span');
  var refreshBtnText = refreshBtn && refreshBtn.querySelector('.btn-text');
  var refreshSpinner = refreshBtn && refreshBtn.querySelector('.spinner');
  var ipv6CopyBtnText = ipv6CopyBtn && ipv6CopyBtn.querySelector('span');
  var ipv6RefreshBtnText = ipv6RefreshBtn && ipv6RefreshBtn.querySelector('.btn-text');
  var ipv6RefreshSpinner = ipv6RefreshBtn && ipv6RefreshBtn.querySelector('.spinner');

  var ipState = 'idle';
  var lastIp = '';
  var lastFetchTime = 0;
  var ipv6State = 'idle';
  var lastIpv6 = '';
  var lastIpv6FetchTime = 0;
  var geoFetched = false;

  function handleAdBar() {
    if (sessionStorage.getItem(AD_SESSION_KEY)) {
      adBar.classList.add('hidden');
    }
    closeAd.addEventListener('click', function () {
      adBar.classList.add('hidden');
      sessionStorage.setItem(AD_SESSION_KEY, '1');
    });
  }

  // HTTP headers are decoded as ISO-8859-1 by the Fetch API, so non-ASCII
  // (e.g. Chinese ad text sent as raw UTF-8 bytes) arrives as mojibake.
  // Re-decode Latin-1 chars back to UTF-8; no-op for already-correct strings.
  function decodeHeaderText(s) {
    if (!s) return s;
    try { return decodeURIComponent(escape(s)); }
    catch (e) { return s; }
  }

  function handleAdHeaders(headers) {
    var enabled = headers.get('X-Ad-Enabled');
    if (enabled === 'true') {
      var text = decodeHeaderText(headers.get('X-Ad-Text'));
      var url = headers.get('X-Ad-URL');
      if (text && url && /^https?:\/\//i.test(url)) {
        adLink.textContent = text;
        adLink.href = url;
        if (!sessionStorage.getItem(AD_SESSION_KEY)) {
          adBar.classList.remove('hidden');
        }
        return;
      }
    }
    adBar.classList.add('hidden');
  }

  function setIpState(state, msg) {
    ipState = state;
    statusMsg.className = 'status-msg';
    statusMsg.textContent = '';

    if (state === 'loading') {
      ipDisplay.textContent = msg || t('loading');
      ipDisplay.className = 'ip-address loading';
      if (refreshBtnText) refreshBtnText.textContent = t('refreshing');
      if (refreshSpinner) refreshSpinner.style.display = 'inline-block';
      refreshBtn.disabled = true;
    } else if (state === 'success') {
      ipDisplay.textContent = lastIp;
      ipDisplay.className = 'ip-address';
      statusMsg.className = 'status-msg success';
      statusMsg.textContent = msg || t('success');
      copyBtn.disabled = false;
      if (refreshBtnText) refreshBtnText.textContent = t('refresh');
      if (refreshSpinner) refreshSpinner.style.display = 'none';
      refreshBtn.disabled = false;
      ipv4Card.classList.remove('loading');
      setTimeout(function () {
        if (ipState === 'success') statusMsg.className = 'status-msg idle';
      }, 2500);
    } else if (state === 'error') {
      ipDisplay.textContent = lastIp || '--';
      ipDisplay.className = 'ip-address';
      statusMsg.className = 'status-msg error';
      statusMsg.textContent = t('error');
      if (refreshBtnText) refreshBtnText.textContent = t('refresh');
      if (refreshSpinner) refreshSpinner.style.display = 'none';
      refreshBtn.disabled = false;
      ipv4Card.classList.remove('loading');
    } else if (state === 'timeout') {
      ipDisplay.textContent = lastIp || '--';
      ipDisplay.className = 'ip-address';
      statusMsg.className = 'status-msg error';
      statusMsg.textContent = t('timeout');
      if (refreshBtnText) refreshBtnText.textContent = t('refresh');
      if (refreshSpinner) refreshSpinner.style.display = 'none';
      refreshBtn.disabled = false;
      ipv4Card.classList.remove('loading');
    } else if (state === 'throttled') {
      statusMsg.className = 'status-msg error';
      statusMsg.textContent = t('throttled');
      setTimeout(function () {
        if (ipState === 'throttled') statusMsg.className = 'status-msg idle';
      }, 2500);
    }
  }

  function fetchGeo(ip) {
    if (geoFetched || !ip) return;
    geoFetched = true;
    fetch(IP4_API, { headers: { 'Accept': 'application/json', 'X-Client': 'web' } })
      .then(function (response) { if (!response.ok) throw new Error('HTTP'); return response.json(); })
      .then(function (data) {
        if (!data || !geoLine) return;
        var parts = [];
        if (data.city) parts.push(data.city);
        if (data.country) parts.push(data.country);
        if (parts.length) geoLine.textContent = parts.join(', ');
      })
      .catch(function () { /* geo is non-critical, fail silently */ });
  }

  function fetchIp() {
    var controller = newAbortController();
    var timeoutId = setTimeout(function () { controller.abort(); setIpState('timeout'); }, FETCH_TIMEOUT_IP4);
    setIpState('loading');
    ipv4Card.classList.add('loading');

    fetch(IP4_API, { headers: { 'X-Client': 'web' }, signal: controller.signal })
      .then(function (response) {
        clearTimeout(timeoutId);
        if (!response.ok) throw new Error('HTTP ' + response.status);
        handleAdHeaders(response.headers);
        return response.text();
      })
      .then(function (text) {
        var ip = text.trim();
        if (!ip) throw new Error('Empty response');
        lastIp = ip;
        lastFetchTime = Date.now();
        setIpState('success');
        fetchGeo(ip);
      })
      .catch(function (err) {
        clearTimeout(timeoutId);
        setIpState(err.name === 'AbortError' ? 'timeout' : 'error');
      });
  }

  function setIpv6Buttons(copyEnabled, refreshEnabled, refreshLoading) {
    if (ipv6CopyBtn) ipv6CopyBtn.disabled = !copyEnabled;
    if (ipv6RefreshBtn) ipv6RefreshBtn.disabled = !refreshEnabled;
    if (ipv6RefreshBtnText) ipv6RefreshBtnText.textContent = t(refreshLoading ? 'refreshing' : 'refresh');
    if (ipv6RefreshSpinner) ipv6RefreshSpinner.style.display = refreshLoading ? 'inline-block' : 'none';
  }

  function setIpv6State(state, ip) {
    ipv6State = state;
    ipv6Status.className = 'status-msg';
    ipv6Status.textContent = '';

    if (state === 'loading') {
      ipv6Address.textContent = t('ipv6_testing');
      ipv6Address.className = 'ip-address loading';
      setIpv6Buttons(false, false, true);
    } else if (state === 'success') {
      ipv6Address.textContent = ip;
      ipv6Address.className = 'ip-address';
      setIpv6Buttons(true, true, false);
      ipv6Card.classList.remove('loading');
      ipv6Card.classList.add('tested');
    } else if (state === 'fail') {
      ipv6Address.textContent = '--';
      ipv6Address.className = 'ip-address loading';
      ipv6Status.className = 'status-msg error';
      ipv6Status.textContent = t('ipv6_not_supported');
      setIpv6Buttons(false, false, false);
      ipv6Card.classList.remove('loading');
      ipv6Card.classList.add('tested');
    }
  }

  function fetchIpv6() {
    if (ipv6State === 'loading') return;

    var controller = newAbortController();
    var timeoutId = setTimeout(function () {
      controller.abort();
      lastIpv6FetchTime = Date.now();
      setIpv6State('fail');
    }, FETCH_TIMEOUT_IP6);

    setIpv6State('loading');
    ipv6Card.classList.add('loading');

    fetch(IP6_API, { headers: { 'X-Client': 'web' }, signal: controller.signal })
      .then(function (response) {
        clearTimeout(timeoutId);
        if (!response.ok) throw new Error('HTTP ' + response.status);
        return response.text();
      })
      .then(function (text) {
        var ip = text.trim();
        if (!ip) throw new Error('Empty response');
        lastIpv6 = ip;
        lastIpv6FetchTime = Date.now();
        setIpv6State('success', ip);
      })
      .catch(function () {
        clearTimeout(timeoutId);
        lastIpv6FetchTime = Date.now();
        setIpv6State('fail');
      });
  }

  function handleIpv6Refresh() {
    if (ipv6State === 'loading') return;
    var now = Date.now();
    if (now - lastIpv6FetchTime < THROTTLE_MS && lastIpv6) {
      ipv6Status.className = 'status-msg error';
      ipv6Status.textContent = t('throttled');
      setTimeout(function () {
        if (ipv6State === 'success') ipv6Status.className = 'status-msg';
      }, 2500);
      return;
    }
    fetchIpv6();
  }

  function newAbortController() {
    try { return new AbortController(); } catch (e) {
      return { signal: null, abort: function () {} };
    }
  }

  function handleRefresh() {
    if (ipState === 'loading') return;
    var now = Date.now();
    if (now - lastFetchTime < THROTTLE_MS && lastIp) {
      setIpState('throttled');
      return;
    }
    fetchIp();
  }

  function showCopied(btnTextEl) {
    if (!btnTextEl) return;
    btnTextEl.textContent = t('copy_success');
    setTimeout(function () { btnTextEl.textContent = t('copy'); }, 1500);
  }

  function fallbackCopy(text, btnTextEl) {
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    try { document.execCommand('copy'); showCopied(btnTextEl); } catch (e) {}
    document.body.removeChild(ta);
  }

  function copyText(text, btnTextEl) {
    if (!text) return;
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(function () { showCopied(btnTextEl); }).catch(function () { fallbackCopy(text, btnTextEl); });
    } else {
      fallbackCopy(text, btnTextEl);
    }
  }

  function handleOnline() {
    if (!lastIp) fetchIp();
  }

  if (copyBtn) copyBtn.addEventListener('click', function () { copyText(lastIp, copyBtnText); });
  if (refreshBtn) refreshBtn.addEventListener('click', handleRefresh);
  if (ipv6CopyBtn) ipv6CopyBtn.addEventListener('click', function () { copyText(lastIpv6, ipv6CopyBtnText); });
  if (ipv6RefreshBtn) ipv6RefreshBtn.addEventListener('click', handleIpv6Refresh);
  window.addEventListener('online', handleOnline);

  handleAdBar();

  fetchIp();
  fetchIpv6();
})();
