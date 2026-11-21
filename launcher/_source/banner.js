// ACCEPT IT 실행기가 메인 화면에 끼워 넣는 '같이 하기 주소' 안내 상자 (이 컴퓨터의 브라우저에서만 보입니다)
(function () {
  var h = location.hostname;
  if (h !== 'localhost' && h !== '127.0.0.1') return; // 참가자 화면에는 표시하지 않음
  var ID = 'acceptit-share';
  var last = '';
  var closed = false;

  function onMain() { return location.pathname === '/' || location.pathname === ''; }
  function remove() { var e = document.getElementById(ID); if (e) e.remove(); last = ''; }

  function copy(text, btn) {
    function done() { var o = btn.textContent; btn.textContent = '복사됨!'; setTimeout(function () { btn.textContent = o; }, 1200); }
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done, function () { fallback(text, done); });
    } else { fallback(text, done); }
  }
  function fallback(text, done) {
    var t = document.createElement('textarea');
    t.value = text; t.style.position = 'fixed'; t.style.opacity = '0';
    document.body.appendChild(t); t.select();
    try { document.execCommand('copy'); done(); } catch (e) { /* 무시 */ }
    t.remove();
  }

  function el(tag, css, text) {
    var e = document.createElement(tag);
    if (css) e.style.cssText = css;
    if (text != null) e.textContent = text;
    return e;
  }

  function linkRow(label, url) {
    var row = el('div', 'display:flex;align-items:center;gap:8px;flex-wrap:wrap;margin-top:6px');
    row.appendChild(el('span', 'white-space:nowrap', label));
    var a = el('a', 'color:inherit;font-weight:700;word-break:break-all', url);
    a.href = url; a.target = '_blank'; a.rel = 'noopener';
    row.appendChild(a);
    var b = el('button', 'font:inherit;cursor:pointer;background:var(--orange,#f7a846);color:inherit;border:3px solid var(--ink,#2b2a33);padding:0 8px;box-shadow:0 3px 0 var(--ink,#2b2a33)', '복사');
    b.type = 'button';
    b.onclick = function () { copy(url, b); };
    row.appendChild(b);
    return row;
  }

  function render(info) {
    var key = JSON.stringify(info);
    if (key === last && document.getElementById(ID)) return;
    last = key;
    var old = document.getElementById(ID); if (old) old.remove();
    var box = el('div',
      'position:fixed;left:16px;bottom:16px;z-index:50;max-width:min(92vw,560px);box-sizing:border-box;' +
      'background:var(--panel,#fbfaf7);color:var(--ink,#2b2a33);border:4px solid var(--ink,#2b2a33);' +
      'box-shadow:4px 4px 0 rgba(0,0,0,.4);padding:10px 14px;font-size:.85rem;line-height:1.5');
    box.id = ID;
    var head = el('div', 'display:flex;justify-content:space-between;gap:12px;align-items:center');
    head.appendChild(el('strong', '', '같이 하려면 이 주소를 알려 주세요'));
    var x = el('button', 'font:inherit;cursor:pointer;background:transparent;border:0;color:inherit;padding:0 4px', '✕');
    x.type = 'button'; x.title = '닫기';
    x.onclick = function () { closed = true; remove(); };
    head.appendChild(x);
    box.appendChild(head);

    if (info.state === 'ready') box.appendChild(linkRow('🌐 인터넷 주소', info.url));
    else if (info.state === 'starting') box.appendChild(el('div', 'margin-top:6px', '🌐 인터넷 주소를 만드는 중… (10~30초 뒤 이 자리에 나옵니다)'));
    else if (info.state === 'missing') box.appendChild(el('div', 'margin-top:6px', '🌐 인터넷 공개를 쓰려면 runtime 폴더에 cloudflared.exe 가 필요합니다.'));
    else if (info.state === 'failed') box.appendChild(el('div', 'margin-top:6px', '🌐 인터넷 주소를 만들지 못했습니다. 인터넷 연결을 확인하고 다시 실행하세요.'));
    else box.appendChild(el('div', 'margin-top:6px', '🌐 다른 와이파이/인터넷으로 하려면 AcceptIt-public.bat 으로 실행하세요.'));

    (info.lan || []).forEach(function (u) { box.appendChild(linkRow('📶 같은 와이파이', u)); });
    document.body.appendChild(box);
  }

  function tick() {
    if (!onMain() || closed) { remove(); return; }
    fetch('/__acceptit/info', { cache: 'no-store' })
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (i) { if (i && onMain() && !closed) render(i); })
      .catch(function () { /* 서버 준비 중 등: 다음 주기에 다시 */ });
  }
  setInterval(tick, 2000);
  tick();
})();
