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
  var copyBtn = document.getElementById('copy-btn');
  var refreshBtn = document.getElementById('refresh-btn');
  var ipv6Btn = document.getElementById('ipv6-btn');
  var ipv6Content = document.getElementById('ipv6-content');
  var ipv4Card = document.getElementById('ipv4-card');
  var ipv6Card = document.getElementById('ipv6-card');
  var adBar = document.getElementById('ad-bar');
  var adLink = document.getElementById('ad-link');
  var closeAd = document.getElementById('close-ad');
  var yearSpan = document.getElementById('year');
  var copyBtnText = copyBtn && copyBtn.querySelector('span');
  var refreshBtnText = refreshBtn && refreshBtn.querySelector('.btn-text');
  var refreshSpinner = refreshBtn && refreshBtn.querySelector('.spinner');

  var ipState = 'idle';
  var lastIp = '';
  var lastFetchTime = 0;
  var ipv6State = 'idle';

  if (yearSpan) yearSpan.textContent = new Date().getFullYear();

  function handleAdBar() {
    if (sessionStorage.getItem(AD_SESSION_KEY)) {
      adBar.classList.add('hidden');
    }
    closeAd.addEventListener('click', function () {
      adBar.classList.add('hidden');
      sessionStorage.setItem(AD_SESSION_KEY, '1');
    });
  }

  function handleAdHeaders(headers) {
    var enabled = headers.get('X-Ad-Enabled');
    if (enabled === 'true') {
      var text = headers.get('X-Ad-Text');
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
      }, 1500);
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
      }, 2000);
    }
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
      })
      .catch(function (err) {
        clearTimeout(timeoutId);
        setIpState(err.name === 'AbortError' ? 'timeout' : 'error');
      });
  }

  function fetchIpv6() {
    if (ipv6State === 'loading') return;

    var controller = newAbortController();
    var timeoutId = setTimeout(function () {
      controller.abort();
      ipv6State = 'fail';
      setIpv6Result('fail', t('ipv6_fail'));
    }, FETCH_TIMEOUT_IP6);

    ipv6State = 'loading';
    setIpv6Result('loading', t('ipv6_checking'));

    fetch(IP6_API, { headers: { 'X-Client': 'web' }, signal: controller.signal })
      .then(function (response) {
        clearTimeout(timeoutId);
        if (!response.ok) throw new Error('HTTP ' + response.status);
        return response.text();
      })
      .then(function (text) {
        var ip = text.trim();
        if (!ip) throw new Error('Empty response');
        ipv6State = 'success';
        setIpv6Result('success', t('ipv6_success').replace('{ip}', ip));
      })
      .catch(function (err) {
        clearTimeout(timeoutId);
        ipv6State = 'fail';
        setIpv6Result('fail', t('ipv6_fail'));
      });
  }

  function setIpv6Result(state, msg) {
    var el = document.getElementById('ipv6-result') || ipv6Content;
    el.className = 'ipv6-result ' + state;
    el.textContent = msg;

    ipv6Card.classList.remove('loading');
    if (state !== 'idle' && state !== 'loading') {
      ipv6Card.classList.add('tested');
    }

    var btn = ipv6Btn;
    if (btn) {
      btn.classList.remove('loading');
      btn.disabled = false;
    }
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

  function handleCopy() {
    if (!lastIp) return;
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(lastIp).then(showCopySuccess).catch(function () { fallbackCopy(lastIp); });
    } else {
      fallbackCopy(lastIp);
    }
  }

  function fallbackCopy(text) {
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    try { document.execCommand('copy'); showCopySuccess(); } catch (e) {}
    document.body.removeChild(ta);
  }

  function showCopySuccess() {
    if (copyBtnText) {
      copyBtnText.textContent = t('copy_success');
      setTimeout(function () { copyBtnText.textContent = t('copy'); }, 1500);
    }
  }

  function handleOnline() {
    if (!lastIp) fetchIp();
  }

  if (copyBtn) copyBtn.addEventListener('click', handleCopy);
  if (refreshBtn) refreshBtn.addEventListener('click', handleRefresh);
  if (ipv6Btn) ipv6Btn.addEventListener('click', function () {
    if (ipv6State === 'loading') return;
    fetchIpv6();
  });
  window.addEventListener('online', handleOnline);

  handleAdBar();

  fetchIp();
  fetchIpv6();
})();
