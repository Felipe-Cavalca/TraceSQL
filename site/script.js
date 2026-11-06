(function () {
  var root = document.documentElement;
  var langBtn = document.getElementById('lang');
  var reduce = window.matchMedia && matchMedia('(prefers-reduced-motion: reduce)').matches;

  function store(k, v) { try { localStorage.setItem(k, v); } catch (e) {} }
  function load(k) { try { return localStorage.getItem(k); } catch (e) { return null; } }

  function setLang(l) {
    root.lang = l;
    langBtn.textContent = l === 'pt' ? 'EN' : 'PT';
    store('tracesql-lang', l);
    relabelDemo();
    setFlowCount(flowN);
  }
  var saved = load('tracesql-lang');
  var initial = saved || (/^pt/i.test(navigator.language || '') ? 'pt' : 'en');
  langBtn.addEventListener('click', function () { setLang(root.lang === 'pt' ? 'en' : 'pt'); });

  // terminal do hero com digitação
  var typed = document.getElementById('typed');
  var script = [
    '$ tracesql --driver postgres --table orders \\\n    --record 10 --depth 2 --ignore-table-suffix _log\n',
    '\n[tracesql] orders: 1 linha\n[tracesql] customers: 1 linha  (pai)\n[tracesql] order_items: 4 linhas (filhos)\n[tracesql] products: 3 linhas\n[tracesql] audit_log: ignorada\n\n-> export_orders_10.sql  (5 tabelas, 9 linhas)\n'
  ];
  if (reduce) {
    typed.textContent = script.join('');
  } else {
    var ti = 0, ts = 0, tout = '';
    (function tick() {
      if (ts >= script.length) { typed.className = ''; return; }
      typed.className = 'cur';
      var chunk = script[ts];
      tout += chunk.charAt(ti++);
      typed.textContent = tout;
      if (ti >= chunk.length) { ts++; ti = 0; setTimeout(tick, 600); return; }
      setTimeout(tick, ts === 0 ? 28 : 6);
    })();
  }

  // ---- motor de seção em tela cheia: palco sticky que troca de cena com o scroll ----
  var scrollies = [];
  var ticking = false;

  function makeScrolly(sec, onScene) {
    var stage = sec.querySelector('.stage');
    var sceneEls = sec.querySelectorAll('.scene-text');
    var dotsBox = sec.querySelector('.dots');
    var N = sceneEls.length, cur = -1, dots = [];
    sec.style.setProperty('--n', N);

    function setScene(i) {
      if (i === cur) return;
      cur = i;
      var k;
      for (k = 0; k < N; k++) sceneEls[k].classList.toggle('on', k === i);
      for (k = 1; k < N; k++) stage.classList.toggle('at' + k, k <= i);
      dots.forEach(function (d, j) { d.classList.toggle('on', j === i); d.classList.toggle('done', j < i); d.setAttribute('aria-selected', j === i); });
      if (onScene) onScene(i);
    }
    function geometry() {
      var hdr = document.querySelector('.top').offsetHeight;
      return { hdr: hdr, top: sec.getBoundingClientRect().top, total: sec.offsetHeight - stage.offsetHeight };
    }
    function update() {
      var g = geometry();
      var p = g.total > 0 ? Math.min(1, Math.max(0, (g.hdr - g.top) / g.total)) : 0;
      setScene(Math.min(N - 1, Math.floor(p * N)));
    }
    for (var d = 0; d < N; d++) {
      (function (i) {
        var b = document.createElement('button');
        b.type = 'button';
        b.setAttribute('role', 'tab');
        b.setAttribute('aria-label', (i + 1) + ' / ' + N);
        b.addEventListener('click', function () {
          var g = geometry();
          var want = ((i + 0.5) / N) * g.total;
          window.scrollBy({ top: want - (g.hdr - g.top), behavior: reduce ? 'auto' : 'smooth' });
        });
        dotsBox.appendChild(b);
        dots.push(b);
      })(d);
    }
    scrollies.push({ update: update });
    setScene(0);
  }
  function updateAll() { ticking = false; scrollies.forEach(function (s) { s.update(); }); }
  function onScroll() { if (!ticking) { ticking = true; requestAnimationFrame(updateAll); } }

  // ---- objetivo: remoto -> .sql -> local ----
  var remoteChips = document.getElementById('remoteChips');
  var localChips = document.getElementById('localChips');
  var pipe = document.querySelector('.pipe');
  var flowCount = document.getElementById('flowCount');
  var names = ['orders', 'customers', 'order_items', 'invoices', 'products', 'audit_log', 'users', 'countries',
    'sessions', 'coupons', 'reports', 'carts', 'tokens', 'newsletters', 'categories', 'jobs'];
  var related = [0, 1, 2, 4, 7];
  var rc = [], lc = [], flowN = 0, flowScene = -1;
  localChips.className = 'chips local';

  names.forEach(function (n, i) {
    var c = document.createElement('span');
    c.className = 'chip' + (i === 0 ? ' base' : '');
    c.textContent = n;
    remoteChips.appendChild(c);
    rc.push(c);
  });
  related.forEach(function (ri, k) {
    rc[ri].style.setProperty('--i', k);
    var c = document.createElement('span');
    c.className = 'chip';
    c.textContent = names[ri];
    c.style.setProperty('--i', k);
    localChips.appendChild(c);
    lc.push(c);
  });

  function setFlowCount(n) {
    flowN = n;
    flowCount.innerHTML = root.lang === 'pt'
      ? '<b>' + n + '</b> de ' + names.length + ' tabelas levadas para o local'
      : '<b>' + n + '</b> of ' + names.length + ' tables brought local';
  }
  function applyFlow(s) {
    related.forEach(function (ri, k) {
      rc[ri].classList.toggle('hit', s >= 1);
      rc[ri].classList.toggle('done', s >= 3);
      lc[k].classList.toggle('in', s >= 2);
    });
    if (s === 2 && flowScene !== 2) { pipe.classList.remove('go'); void pipe.offsetWidth; pipe.classList.add('go'); }
    if (s < 2) pipe.classList.remove('go');
    setFlowCount(s >= 2 ? related.length : 0);
    flowScene = s;
  }
  makeScrolly(document.getElementById('objetivo'), applyFlow);

  // ---- como funciona: grafo SVG ----
  var svg = document.getElementById('graphSvg');
  var NS = 'http://www.w3.org/2000/svg';
  function el(name, attrs, parent) {
    var e = document.createElementNS(NS, name);
    for (var k in attrs) e.setAttribute(k, attrs[k]);
    if (parent) parent.appendChild(e);
    return e;
  }
  var cols = [95, 290, 485], rows = [45, 135, 225, 315], W = 112, H = 34;
  var nodes = [
    { n: 'countries', c: 0, r: 0, rel: 2 }, { n: 'jobs', c: 1, r: 0 }, { n: 'products', c: 2, r: 0, rel: 2 },
    { n: 'customers', c: 0, r: 1, rel: 1 }, { n: 'orders', c: 1, r: 1, base: true }, { n: 'order_items', c: 2, r: 1, rel: 1 },
    { n: 'users', c: 0, r: 2 }, { n: 'shipments', c: 1, r: 2, rel: 1 }, { n: 'order_log', c: 2, r: 2, ign: true },
    { n: 'sessions', c: 0, r: 3 }, { n: 'coupons', c: 1, r: 3 }, { n: 'reports', c: 2, r: 3 }
  ];
  var links = [
    ['customers', 'orders', 1], ['orders', 'order_items', 1], ['orders', 'shipments', 1],
    ['orders', 'order_log', 0, true], ['countries', 'customers', 2], ['order_items', 'products', 2]
  ];
  var byName = {};
  nodes.forEach(function (n) { n.x = cols[n.c]; n.y = rows[n.r]; byName[n.n] = n; });
  var linkLayer = el('g', {}, svg);
  links.forEach(function (l) {
    var a = byName[l[0]], b = byName[l[1]];
    var p = el('path', { d: 'M' + a.x + ' ' + a.y + ' L' + b.x + ' ' + b.y, pathLength: 1, 'class': 'link' + (l[3] ? ' ign' : '') }, linkLayer);
    p.style.setProperty('--dl', (l[2] * 0.3) + 's');
  });
  nodes.forEach(function (n) {
    var g = el('g', { 'class': 'node' + (n.base ? ' base' : '') + (n.rel ? ' rel' : '') + (n.ign ? ' ign' : '') }, svg);
    if (n.rel) g.style.setProperty('--dl', (n.rel * 0.3 + 0.1) + 's');
    el('rect', { x: n.x - W / 2, y: n.y - H / 2, width: W, height: H, rx: 7 }, g);
    var t = el('text', { x: n.x, y: n.y }, g);
    t.textContent = n.n;
    if (n.base) {
      var b = el('text', { x: n.x, y: n.y - H / 2 - 8, 'class': 'badge' }, svg);
      b.textContent = 'id = 10';
    }
  });
  makeScrolly(document.getElementById('como'));

  // ---- teste a profundidade ----
  var tables = [
    { name: 'orders',      d: 0, rows: 1, tag: { pt: 'base',            en: 'base' } },
    { name: 'customers',   d: 1, rows: 1, tag: { pt: 'pai · nível 1',   en: 'parent · level 1' } },
    { name: 'order_items', d: 1, rows: 4, tag: { pt: 'filho · nível 1', en: 'child · level 1' } },
    { name: 'shipments',   d: 1, rows: 1, tag: { pt: 'filho · nível 1', en: 'child · level 1' } },
    { name: 'order_log',   d: 1, rows: 6, tag: { pt: 'filho · nível 1', en: 'child · level 1' }, log: true },
    { name: 'products',    d: 2, rows: 3, tag: { pt: 'pai · nível 2',   en: 'parent · level 2' } },
    { name: 'countries',   d: 2, rows: 1, tag: { pt: 'pai · nível 2',   en: 'parent · level 2' } },
    { name: 'categories',  d: 3, rows: 2, tag: { pt: 'pai · nível 3',   en: 'parent · level 3' } }
  ];
  var totalRows = tables.reduce(function (s, t) { return s + t.rows; }, 0);
  var depth = document.getElementById('depth');
  var depthOut = document.getElementById('depthOut');
  var skip = document.getElementById('skipLog');
  var graph = document.getElementById('graph');
  var count = document.getElementById('count');
  var meter = document.getElementById('meterFill');
  var items = [], prevDepth = +depth.value;
  var shown = { r: 0, t: 0 }, target = { r: 0, t: 0 }, raf = 0;

  tables.forEach(function (t) {
    var li = document.createElement('li');
    li.style.setProperty('--d', t.d);
    li.innerHTML = '<span class="name"></span><span class="tag"></span><span class="n"></span>';
    li.children[0].textContent = t.name;
    graph.appendChild(li);
    items.push({ li: li, t: t, state: null });
  });

  function relabelDemo() {
    var l = root.lang;
    items.forEach(function (it) {
      it.li.children[1].textContent = it.t.tag[l];
      it.li.children[2].textContent = it.t.rows + (l === 'pt' ? (it.t.rows > 1 ? ' linhas' : ' linha') : (it.t.rows > 1 ? ' rows' : ' row'));
    });
    paintCount();
  }
  function paintCount() {
    var r = Math.round(shown.r), t = Math.round(shown.t);
    count.innerHTML = root.lang === 'pt'
      ? 'no arquivo: <b>' + r + '</b> de ' + totalRows + ' linhas · <b>' + t + '</b> de ' + tables.length + ' tabelas'
      : 'in the file: <b>' + r + '</b> of ' + totalRows + ' rows · <b>' + t + '</b> of ' + tables.length + ' tables';
  }
  function tweenCount() {
    cancelAnimationFrame(raf);
    if (reduce) { shown.r = target.r; shown.t = target.t; paintCount(); return; }
    var r0 = shown.r, t0 = shown.t, start = performance.now();
    (function f(now) {
      var p = Math.min(1, (now - start) / 380);
      p = 1 - Math.pow(1 - p, 3);
      shown.r = r0 + (target.r - r0) * p;
      shown.t = t0 + (target.t - t0) * p;
      paintCount();
      if (p < 1) raf = requestAnimationFrame(f);
    })(start);
  }

  function renderDemo() {
    var d = +depth.value, growing = d >= prevDepth, rowsIn = 0, tabsIn = 0;
    depthOut.textContent = d;
    depth.style.setProperty('--p', (d / 3 * 100) + '%');
    if (d !== prevDepth) {
      depthOut.classList.remove('bump'); void depthOut.offsetWidth; depthOut.classList.add('bump');
    }
    items.forEach(function (it) {
      var inc = it.t.d <= d && !(skip.checked && it.t.log);
      var st = it.t.d === 0 ? 'base' : inc ? 'in' : 'out';
      if (st === 'in' || st === 'base') { rowsIn += it.t.rows; tabsIn++; }
      if (st === it.state) return;
      var first = it.state === null;
      it.state = st;
      it.li.classList.remove('base', 'in', 'out', 'enter', 'leave');
      it.li.classList.add(st);
      if (!first && !reduce) {
        // a mudança se propaga a partir da base: para fora ao aumentar, para dentro ao diminuir
        var delay = growing ? it.t.d * 90 : (3 - it.t.d) * 60;
        it.li.style.setProperty('--dl', delay + 'ms');
        void it.li.offsetWidth;
        it.li.classList.add(st === 'in' ? 'enter' : 'leave');
      }
    });
    target.r = rowsIn; target.t = tabsIn;
    meter.style.width = (rowsIn / totalRows * 100) + '%';
    tweenCount();
    prevDepth = d;
  }
  depth.addEventListener('input', renderDemo);
  skip.addEventListener('change', renderDemo);
  renderDemo();

  // o scroll leva a bolinha para a profundidade da cena; a mão do usuário vale até a próxima cena
  makeScrolly(document.getElementById('demo'), function (i) {
    depth.value = i;
    renderDemo();
  });

  makeScrolly(document.getElementById('usar'));

  root.classList.add('sc');
  window.addEventListener('scroll', onScroll, { passive: true });
  window.addEventListener('resize', onScroll);
  updateAll();
  setLang(initial);
})();
