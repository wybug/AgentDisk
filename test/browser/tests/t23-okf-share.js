/**
 * T23: OKF Bundle 分享 — 端到端
 *
 * 用法: node runner.js t23
 *
 * 前置条件:
 *   - dev 栈运行中 (bash scripts/dev.sh start)
 *
 * 测试覆盖 P4a plan Step 11：
 *   1. seed demo bundle（在 t02f 全清后必须重 seed）
 *   2. 通过 API 创建 bundle share（无 extractCode）
 *   3. 无 cookie 访问 /share/:code → 渲染 OkfShareBundleView
 *   4. 2 个 tab（节点 / 图谱）
 *   5. 图谱 tab → Cytoscape canvas 存在
 *   6. 节点 tab 行点击 → drawer
 *   7. drawer 点 [预览 Markdown] → modal 渲染 markdown
 *   8. 没有 刷新 / 重建 / 注销 按钮（read-only 验证）
 *   9. 负向：无 auth 调 /v1/disk/okf/bundles/:id → 401
 *  10. cleanup: 撤销 share
 */
const { execSync } = require('child_process');
const path = require('path');
const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

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

function unwrap(s) {
  if (typeof s !== 'string') return s;
  if (s.length >= 2 && s.charAt(0) === '"' && s.charAt(s.length - 1) === '"') {
    return s.slice(1, -1).replace(/\\"/g, '"').replace(/\\\\/g, '\\');
  }
  return s;
}

function createBundleShareAPI(bundleId, extractCode, maxVisit, expireHours) {
  return unwrap(ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/shares', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          resourceId: ${bundleId},
          resType: 'bundle',
          extractCode: '${extractCode}',
          maxVisit: ${maxVisit},
          expireHours: ${expireHours}
        }),
        credentials: 'include'
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: id=' + d.data.id + ' code=' + d.data.shareCode;
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `));
}

function revokeShareAPI(shareId) {
  return unwrap(ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/shares', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ shareId: ${shareId} }),
        credentials: 'include'
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: revoked';
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `));
}

function publicShareBundleAPI(code, bundleId) {
  return unwrap(ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/share/${code}/bundle?bundleId=${bundleId}', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: title=' + (d.data.title || 'none');
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `));
}

function accessShareAPI(code) {
  return unwrap(ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/share/access', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: '${code}' })
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: resType=' + d.data.resType + ' id=' + d.data.resourceId;
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
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

describe('T23: OKF Bundle 分享端到端', () => {
  step('T23.0: seed demo bundle', SEED_BUNDLE_ID !== null, 'bundleId=' + SEED_BUNDLE_ID);

  ab.closeAll();
  ab.login('user001', 'test123');
  ab.waitMs(2000);

  // T23.1 - 创建 bundle share（无 extractCode）
  const createRes = createBundleShareAPI(SEED_BUNDLE_ID, '', 100, 72);
  ab.waitMs(1000);
  const createOk = createRes.startsWith('OK:');
  assertCondition(createOk, 'T23.1: 创建 bundle share 成功', 'resp=' + createRes);
  const codeMatch = createRes.match(/code=([a-zA-Z0-9]+)/);
  const idMatch = createRes.match(/id=(\d+)/);
  const shareCode = codeMatch ? codeMatch[1] : null;
  const shareId = idMatch ? idMatch[1] : null;
  step('T23.1b: 解析 shareCode', shareCode !== null, 'code=' + shareCode);

  // T23.2 - 公开端点：GET /share/:code/bundle 返回 bundle metadata
  const pubRes = publicShareBundleAPI(shareCode, SEED_BUNDLE_ID);
  ab.waitMs(1000);
  assertCondition(pubRes.startsWith('OK:'), 'T23.2: 公开 bundle 端点可读', 'resp=' + pubRes);

  // T23.3 - access endpoint 确认 resType=bundle
  const accessRes = accessShareAPI(shareCode);
  ab.waitMs(1000);
  assertCondition(
    accessRes.includes('OK:') && accessRes.includes('resType=bundle'),
    'T23.3: access share 返回 resType=bundle',
    'resp=' + accessRes,
  );

  // T23.4 - 负向：无 auth 调 HybridAuth 端点 → 401
  const negativeRes = unwrap(ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/okf/bundles/${SEED_BUNDLE_ID}', { credentials: 'omit' })
        .then(function(r) { return 'status=' + r.status; })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `));
  ab.waitMs(1000);
  assertCondition(
    negativeRes.includes('status=401'),
    'T23.4: 无 auth 调用 OKF 端点被拒（401）',
    'resp=' + negativeRes,
  );

  // T23.5 - 关闭登录 session，以无 cookie 方式访问 /share/:code
  ab.closeAll();
  ab.open(ab.BASE_URL + '/share/' + shareCode);
  ab.waitMs(2500);
  ab.waitLoad('networkidle');
  const shareUrl = ab.getUrl();
  assertCondition(
    shareUrl.includes('/share/' + shareCode),
    'T23.5: 访问 /share/:code 页面加载',
    'url=' + shareUrl,
  );
  ab.screenshot('t23-01-share-access-page');

  // 点 [访问] 按钮（无 extractCode 直接验证通过）
  ab.evalStdin(`
    (function() {
      var btn = document.querySelector('button.ant-btn-primary');
      if (btn) { btn.click(); return 'clicked'; }
      return 'no-btn';
    })()
  `);
  ab.waitMs(3000);
  ab.waitLoad('networkidle');
  ab.screenshot('t23-02-share-bundle-view');

  // T23.6 - OkfShareBundleView 渲染：标题 + 2 个 tab
  const bundleTitleVisible = ab.pageContainsText('Bundle') || ab.pageContainsText('OKF');
  assertCondition(
    bundleTitleVisible,
    'T23.6: OkfShareBundleView 渲染（bundle 标题可见）',
    '',
  );

  // T23.7 - 2 个 tab（节点 / 图谱）；统计 / 死链 应该不出现
  const hasNodesTab = ab.pageContainsText('节点');
  const hasGraphTab = ab.pageContainsText('图谱');
  const hasStatsTab = ab.pageContainsText('统计');
  const hasBrokenTab = ab.pageContainsText('死链');
  step(
    'T23.7a: 节点 / 图谱 tab 存在',
    hasNodesTab && hasGraphTab,
    'nodes=' + hasNodesTab + ' graph=' + hasGraphTab,
  );
  step(
    'T23.7b: 统计 / 死链 tab 不存在（简化视图）',
    !hasStatsTab && !hasBrokenTab,
    'stats=' + hasStatsTab + ' broken=' + hasBrokenTab,
  );

  // T23.8 - 没有 刷新 / 重建 / 注销 按钮（read-only 验证）
  const hasRefreshBtn = ab.pageContainsText('刷新 Bundle');
  const hasRebuildBtn = ab.pageContainsText('重建索引');
  const hasUnregisterBtn = ab.pageContainsText('注销');
  step(
    'T23.8: read-only — 无刷新 / 重建 / 注销按钮',
    !hasRefreshBtn && !hasRebuildBtn && !hasUnregisterBtn,
    'refresh=' + hasRefreshBtn + ' rebuild=' + hasRebuildBtn + ' unregister=' + hasUnregisterBtn,
  );

  // T23.9 - 节点 tab（默认）：等表格渲染
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
  assertCondition(
    Number(nodeRows) > 0,
    'T23.9: 节点 tab 表格有数据',
    'rows=' + nodeRows,
  );
  ab.screenshot('t23-03-nodes-tab');

  // T23.10 - 行点击 → drawer
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
  assertCondition(
    drawerOpen === 'open',
    'T23.10: 行点击打开 frontmatter drawer',
    'drawer=' + drawerOpen,
  );
  ab.screenshot('t23-04-frontmatter-drawer');

  // T23.11 - drawer 点 [预览 Markdown] → modal 渲染 markdown
  ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('.ant-drawer-content button, .ant-drawer button');
      for (var i = 0; i < btns.length; i++) {
        var t = btns[i].textContent || '';
        if (t.includes('预览 Markdown')) { btns[i].click(); return 'clicked'; }
      }
      return 'no-preview-btn';
    })()
  `);
  ab.waitMs(2000);
  const modalOpen = unwrap(ab.evalStdin(`
    (function() {
      var modals = document.querySelectorAll('.ant-modal');
      for (var i = 0; i < modals.length; i++) {
        var rect = modals[i].getBoundingClientRect();
        if (rect.width > 0 && rect.height > 0) return 'open';
      }
      return 'closed';
    })()
  `));
  assertCondition(
    modalOpen === 'open',
    'T23.11: 预览按钮打开 markdown modal',
    'modal=' + modalOpen,
  );
  ab.screenshot('t23-05-markdown-modal');

  // T23.12 - modal 内 markdown 渲染（.markdown-body 存在）
  let markdownRendered = false;
  for (let w = 0; w < 5; w++) {
    ab.waitMs(1000);
    const hasMarkdown = unwrap(ab.evalStdin(`
      (function() {
        var body = document.querySelectorAll('.ant-modal .markdown-body, .ant-modal-body .markdown-body');
        return String(body.length);
      })()
    `));
    if (Number(hasMarkdown) > 0) {
      markdownRendered = true;
      break;
    }
  }
  assertCondition(
    markdownRendered,
    'T23.12: modal 内渲染了 markdown body',
    '',
  );
  ab.screenshot('t23-06-markdown-rendered');

  // 关闭 modal
  ab.evalStdin(`
    (function() {
      var close = document.querySelector('.ant-modal-close');
      if (close) close.click();
      return 'closed';
    })()
  `);
  ab.waitMs(800);

  // T23.13 - 切到图谱 tab → Cytoscape canvas
  clickTab('图谱');
  ab.waitMs(3000);
  ab.waitLoad('networkidle');
  const canvasCount = unwrap(ab.evalStdin(`
    (function() {
      var canvases = document.querySelectorAll('canvas');
      return String(canvases.length);
    })()
  `));
  assertCondition(
    Number(canvasCount) > 0,
    'T23.13: 图谱 tab 渲染 Cytoscape canvas',
    'canvas=' + canvasCount,
  );
  ab.screenshot('t23-07-graph-tab');

  // T23.14 - 切回节点 tab
  clickTab('节点');
  ab.waitMs(1500);
  const backToNodes = ab.pageContainsText('节点');
  step('T23.14: 切回节点 tab', backToNodes, '');

  // T23.15 - cleanup：重新登录撤销 share
  ab.closeAll();
  ab.login('user001', 'test123');
  ab.waitMs(2000);
  const revokeRes = revokeShareAPI(shareId);
  ab.waitMs(1000);
  assertCondition(
    revokeRes.includes('OK'),
    'T23.15: cleanup 撤销 share',
    'resp=' + revokeRes,
  );

  ab.closeBrowser();
});
