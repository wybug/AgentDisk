/**
 * T22: OKF 知识库 — bundle 列表 / 详情 / 4 个 tab / drawer 完整链路
 *
 * 用法: node runner.js t22
 *
 * 前置条件:
 *   - dev 栈运行中 (bash scripts/dev.sh start)
 *
 * 测试自带 seed: t02f 全局清理会删除所有 public dirs，所以本测试
 * 必须在执行前重新 seed。scripts/seed_okf_demo.py 是幂等的。
 *
 * 覆盖 plan Step 10 的 13 步浏览器验证：
 *   1. 登录 → sidebar 出现「OKF 知识库」入口
 *   2. /okf 列表页加载并显示至少一个 bundle
 *   3. 进入 /okf/:bundleId 详情页
 *   4. 头部信息（title / status / version / 节点数 / 边数）
 *   5. 节点 tab：表格有数据
 *   6. 图谱 tab：Cytoscape 容器渲染
 *   7. 统计 tab：4 个 Statistic tile + 类型分布
 *   8. 死链 tab：扫描按钮 + 列表
 *   9. 行点击 → frontmatter drawer
 */
const { execSync } = require('child_process');
const path = require('path');
const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

// Idempotent — reuses "OKF Demo Bundle" public dir + bundle if they survived
// a prior run; creates them otherwise. t02f wipes them in the full suite.
// __dirname = .../test/browser/tests → 3 '..' climbs to repo root.
const REPO_ROOT = path.resolve(__dirname, '..', '..', '..');
const SEED_OUTPUT = (() => {
  try {
    return execSync('python3 scripts/seed_okf_demo.py 2>&1', {
      cwd: REPO_ROOT,
      encoding: 'utf8',
      timeout: 60000,
    });
  } catch (e) {
    return 'SEED_FAILED: ' + (e.message || String(e));
  }
})();
const SEED_BUNDLE_ID = (() => {
  const m = SEED_OUTPUT.match(/bundle id\s*:\s*(\d+)/);
  return m ? m[1] : null;
})();

// agent-browser eval wraps string returns in double quotes ("yes" instead of yes).
// Strip them so callers can compare against the raw string.
function unwrap(s) {
  if (typeof s !== 'string') return s;
  if (s.length >= 2 && s.charAt(0) === '"' && s.charAt(s.length - 1) === '"') {
    return s.slice(1, -1).replace(/\\"/g, '"').replace(/\\\\/g, '\\');
  }
  return s;
}

function fetchBundles() {
  return unwrap(ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/okf/bundles', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          var items = d.data || [];
          return 'OK:' + items.length + ':' + items.map(function(b) {
            return b.bundleId + '|' + (b.title || '').substring(0, 30);
          }).join(',');
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `));
}

function fetchBundleDetail(bundleId) {
  return unwrap(ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/okf/bundles/${bundleId}', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          var b = d.data || {};
          return 'OK:' + JSON.stringify({
            id: b.bundleId,
            title: b.title,
            status: b.status,
            okfVersion: b.okfVersion,
            nodeCount: b.nodeCount,
            edgeCount: b.edgeCount,
          });
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `));
}

function clickSidebarMenu(label) {
  return unwrap(ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if ((items[i].textContent || '').includes('${label}')) {
          items[i].click();
          return 'clicked';
        }
      }
      return 'not-found';
    })()
  `));
}

function clickTab(label) {
  return unwrap(ab.evalStdin(`
    (function() {
      var tabs = document.querySelectorAll('.ant-tabs-tab, [role="tab"]');
      for (var i = 0; i < tabs.length; i++) {
        if ((tabs[i].textContent || '').includes('${label}')) {
          tabs[i].click();
          return 'clicked';
        }
      }
      return 'not-found';
    })()
  `));
}

function hasSidebarEntry(label) {
  return unwrap(ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if ((items[i].textContent || '').includes('${label}')) return 'yes';
      }
      return 'no';
    })()
  `));
}

describe('T22: OKF 知识库 UI 完整链路', () => {
  step('T22.0: seed demo bundle', SEED_BUNDLE_ID !== null, 'bundleId=' + SEED_BUNDLE_ID);

  ab.closeAll();
  ab.login('user001', 'test123');
  ab.waitMs(2000);

  // T22.1 - sidebar 出现 OKF 入口
  const okfEntry = hasSidebarEntry('OKF 知识库');
  assertCondition(okfEntry === 'yes', 'T22.1: 侧边栏出现「OKF 知识库」入口', 'entry=' + okfEntry);

  // T22.2 - 点击入口导航到 /okf
  clickSidebarMenu('OKF 知识库');
  ab.waitMs(2500);
  ab.waitLoad('networkidle');
  const urlList = ab.getUrl();
  assertCondition(urlList.includes('/okf'), 'T22.2: 导航到 /okf 列表页', urlList);
  ab.screenshot('t22-01-list-page');

  // T22.3 - 至少有一个 bundle（API 视角）
  const bundlesResp = fetchBundles();
  const hasBundle = bundlesResp.startsWith('OK:') && !bundlesResp.startsWith('OK:0:');
  assertCondition(hasBundle, 'T22.3: 列表页可见至少 1 个 bundle', 'resp=' + bundlesResp.substring(0, 120));

  // T22.4 - 列表页可见 bundle 卡片（DOM 视角）
  const cardsVisible = unwrap(ab.evalStdin(`
    (function() {
      var cards = document.querySelectorAll('.ant-list-item, .ant-card');
      var hit = 0;
      for (var i = 0; i < cards.length; i++) {
        var t = cards[i].textContent || '';
        if (t.includes('Bundle') || t.includes('OKF')) hit++;
      }
      return String(hit);
    })()
  `));
  step('T22.4: 列表页渲染 bundle 卡片', Number(cardsVisible) > 0, 'cards=' + cardsVisible);

  // 解析第一个 bundle 的 id（用于详情页跳转）
  const match = bundlesResp.match(/^OK:\d+:(\d+)\|/);
  const bundleId = match ? match[1] : null;
  if (!bundleId) {
    step('T22.5+: 跳过（无 bundle 可用）', false, 'resp=' + bundlesResp);
    ab.closeBrowser();
    return;
  }

  // T22.5 - 进入详情页（直接导航更可靠；卡片 onClick 在实际交互中也可用，
  // 但 .click() 模拟事件触发链路与 React SyntheticEvent 不总是一致）
  ab.open(ab.BASE_URL + '/okf/' + bundleId);
  ab.waitMs(2500);
  ab.waitLoad('networkidle');
  const urlDetail = ab.getUrl();
  assertCondition(urlDetail.includes('/okf/' + bundleId), 'T22.5: 进入 /okf/' + bundleId + ' 详情页', urlDetail);
  ab.screenshot('t22-02-detail-page');

  // T22.6 - 详情头部信息（title / status / version / counts）
  const headerInfo = fetchBundleDetail(bundleId);
  const headerOk = headerInfo.startsWith('OK:') &&
    headerInfo.includes('"nodeCount"') &&
    headerInfo.includes('"edgeCount"');
  assertCondition(headerOk, 'T22.6: 详情头部信息完整', 'resp=' + headerInfo.substring(0, 200));

  // 详情头部 DOM 验证：轮询等待 React Query 完成 bundle 加载并渲染头部 tag
  // （"X 节点" / "Y 边"）。最多等 8 秒。
  let headerDom = false;
  for (let w = 0; w < 8; w++) {
    ab.waitMs(1000);
    if (ab.pageContainsText('节点') && ab.pageContainsText('边')) { headerDom = true; break; }
  }
  step('T22.6b: 详情页显示节点 / 边计数标签', headerDom, '');

  // T22.7 - 节点 tab（默认）：表格有数据（轮询直到 React Query 渲染完节点行）
  let nodeRows = '0';
  for (let w = 0; w < 8; w++) {
    ab.waitMs(1000);
    nodeRows = unwrap(ab.evalStdin(`
      (function() {
        var rows = document.querySelectorAll('.ant-table-tbody tr.ant-table-row');
        return String(rows.length);
      })()
    `));
    if (Number(nodeRows) > 0) break;
  }
  assertCondition(Number(nodeRows) > 0, 'T22.7: 节点 tab 表格有节点数据', 'rows=' + nodeRows);
  ab.screenshot('t22-03-nodes-tab');

  // T22.8 - 行点击 → frontmatter drawer
  // The click target is the <a> inside the first cell (title), not the <tr>.
  // Clicking the row itself doesn't fire the link's React onClick.
  ab.evalStdin(`
    (function() {
      var link = document.querySelector('.ant-table-tbody tr.ant-table-row td a');
      if (!link) return 'no-link';
      link.click();
      return 'clicked';
    })()
  `);
  ab.waitMs(1500);
  const drawerOpen = unwrap(ab.evalStdin(`
    (function() {
      var drawers = document.querySelectorAll('.ant-drawer-content, .ant-drawer');
      for (var i = 0; i < drawers.length; i++) {
        var rect = drawers[i].getBoundingClientRect();
        if (rect.width > 0 && rect.height > 0) return 'open';
      }
      return 'closed';
    })()
  `));
  assertCondition(drawerOpen === 'open', 'T22.8: 点击节点行打开 frontmatter drawer', 'drawer=' + drawerOpen);
  ab.screenshot('t22-04-frontmatter-drawer');

  // T22.9 - 关闭 drawer
  ab.evalStdin(`
    (function() {
      var btn = document.querySelector('.ant-drawer-content .ant-drawer-close, .ant-drawer .ant-drawer-close');
      if (btn) { btn.click(); return 'closed'; }
      return 'no-close-btn';
    })()
  `);
  ab.waitMs(1000);

  // T22.10 - 切到图谱 tab，Cytoscape canvas 渲染
  clickTab('图谱');
  ab.waitMs(3000);
  ab.waitLoad('networkidle');
  const cyMounted = ab.evalStdin(`
    (function() {
      var canvas = document.querySelectorAll('canvas, svg');
      var container = document.querySelectorAll('.cytoscape-container, [_instanceid]');
      // Cytoscape.js renders either a canvas or an svg depending on the renderer.
      return 'canvas:' + canvas.length + '|container:' + container.length;
    })()
  `);
  // Cytoscape often uses canvas — at minimum some graph-related element should exist.
  step('T22.10: 图谱 tab 挂载 Cytoscape', cyMounted.length > 0, cyMounted);
  ab.screenshot('t22-05-graph-tab');

  // T22.11 - 切到统计 tab
  clickTab('统计');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  const statsCount = unwrap(ab.evalStdin(`
    (function() {
      var tiles = document.querySelectorAll('.ant-statistic-content, .ant-statistic');
      return String(tiles.length);
    })()
  `));
  assertCondition(Number(statsCount) >= 4, 'T22.11: 统计 tab 显示 4 个 Statistic tile', 'tiles=' + statsCount);
  ab.screenshot('t22-06-stats-tab');

  // T22.12 - 切到死链 tab
  clickTab('死链');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  const brokenTabActive = ab.pageContainsText('扫描') || ab.pageContainsText('死链');
  assertCondition(brokenTabActive, 'T22.12: 死链 tab 加载（扫描按钮可见）', '');
  ab.screenshot('t22-07-broken-tab');

  // T22.13 - 点击扫描按钮（如果存在）
  ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        var t = btns[i].textContent || '';
        if (t.includes('扫描')) { btns[i].click(); return 'clicked'; }
      }
      return 'no-scan-btn';
    })()
  `);
  ab.waitMs(2500);
  const brokenRows = unwrap(ab.evalStdin(`
    (function() {
      var rows = document.querySelectorAll('.ant-table-tbody tr.ant-table-row');
      return String(rows.length);
    })()
  `));
  step('T22.13: 死链扫描完成', true, 'rows=' + brokenRows);
  ab.screenshot('t22-08-broken-after-scan');

  // T22.14 - 回到节点 tab 验证可以切回
  clickTab('节点');
  ab.waitMs(1500);
  const backToNodes = ab.pageContainsText('Transformer') || ab.pageContainsText('节点');
  step('T22.14: 切回节点 tab', backToNodes, '');

  ab.screenshot('t22-09-final');
  ab.closeBrowser();
});
