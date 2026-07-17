/**
 * T25: OKF 搜索 UI — bundle 内全文搜索端到端
 *
 * 用法: node runner.js t25
 *
 * 前置条件:
 *   - dev 栈运行中 (bash scripts/dev.sh start)
 *
 * 覆盖 okfApi.search 的前端接入（此前该 API 零调用点，知识库无法搜索）：
 *   1. seed demo bundle
 *   2. 详情页渲染「搜索节点」面板
 *   3. 输入命中关键词 → 节点结果行渲染
 *   4. 点击结果行 → 打开 frontmatter drawer
 *   5. 清除 → 结果清空
 *   6. 搜索无匹配词 → 空态「没有匹配的节点」
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

// triggerSearch finds the search input by placeholder, sets the value the
// React-aware way, then triggers onSearch via the search button (AntD v5:
// .ant-input-search-button) — falling back to an Enter keypress. We avoid
// text-matching the "搜索" button (the node-list has its own) and log the
// outcome so a missing input/button is diagnosable.
function triggerSearch(term) {
  const res = unwrap(ab.evalStdin(`
    (function() {
      var inputs = document.querySelectorAll('input');
      var input = null;
      for (var i = 0; i < inputs.length; i++) {
        if (inputs[i].placeholder && inputs[i].placeholder.indexOf('搜索节点标题') !== -1) { input = inputs[i]; break; }
      }
      if (!input) {
        var allInputs = document.querySelectorAll('input');
        var phs = [];
        for (var j = 0; j < allInputs.length; j++) {
          phs.push('type=' + (allInputs[j].type || '?') + '|ph=' + (allInputs[j].placeholder || '(none)'));
        }
        var cards = document.querySelectorAll('.ant-card');
        var panelHtml = '(no-card-with-搜索节点)';
        for (var k = 0; k < cards.length; k++) {
          if ((cards[k].textContent || '').indexOf('搜索节点') !== -1) {
            panelHtml = cards[k].outerHTML.slice(0, 600);
            break;
          }
        }
        return 'no-input|allInputs=[' + phs.join('] [') + ']|panel=' + panelHtml;
      }
      var nativeSet = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
      nativeSet.call(input, '${term}');
      input.dispatchEvent(new Event('input', { bubbles: true }));
      input.dispatchEvent(new Event('change', { bubbles: true }));
      input.focus();
      // Enter's onKeyDown reads input.value (DOM) directly, unlike the search
      // button which reads React state — so Enter fires onSearch with the value
      // we just set even when the React state tracker didn't catch the change.
      var opts = { key: 'Enter', code: 'Enter', keyCode: 13, which: 13, bubbles: true, cancelable: true };
      input.dispatchEvent(new KeyboardEvent('keydown', opts));
      input.dispatchEvent(new KeyboardEvent('keypress', opts));
      input.dispatchEvent(new KeyboardEvent('keyup', opts));
      return 'enter:' + input.value;
    })()
  `));
  return res;
}

function listRowCount() {
  return unwrap(ab.evalStdin(`
    (function() { return String(document.querySelectorAll('.ant-list-item').length); })()
  `));
}

function drawerOpen() {
  return unwrap(ab.evalStdin(`
    (function() {
      var ds = document.querySelectorAll('.ant-drawer-content, .ant-drawer');
      for (var i = 0; i < ds.length; i++) {
        var rect = ds[i].getBoundingClientRect();
        if (rect.width > 0 && rect.height > 0) return 'open';
      }
      return 'closed';
    })()
  `));
}

describe('T25: OKF 搜索 UI 端到端', () => {
  step('T25.0: seed demo bundle', SEED_BUNDLE_ID !== null, 'bundleId=' + SEED_BUNDLE_ID);

  ab.closeAll();
  ab.login('user001', 'test123');
  ab.waitMs(2000);

  ab.open(ab.BASE_URL + '/okf/' + SEED_BUNDLE_ID);
  ab.waitMs(2500);
  ab.waitLoad('networkidle');

  assertCondition(ab.pageContainsText('搜索节点'), 'T25.1: 详情页渲染搜索面板', '');
  ab.screenshot('t25-01-search-panel');

  // T25.2 - 搜索命中词（demo bundle 含「Transformer 架构」节点）
  triggerSearch('Transformer');
  ab.waitMs(2500);
  ab.waitLoad('networkidle');
  const rows = listRowCount();
  assertCondition(Number(rows) > 0, 'T25.2: 搜索命中返回节点行', 'rows=' + rows);
  ab.screenshot('t25-02-search-results');

  // T25.3 - 点击首行结果 → drawer
  ab.evalStdin(`
    (function() {
      var r = document.querySelector('.ant-list-item');
      if (r) { r.click(); return 'clicked'; }
      return 'no-row';
    })()
  `);
  ab.waitMs(1500);
  assertCondition(drawerOpen() === 'open', 'T25.3: 点击结果打开 frontmatter drawer', '');
  ab.screenshot('t25-03-result-drawer');
  // 关闭 drawer
  ab.evalStdin(`
    (function() {
      var c = document.querySelector('.ant-drawer-close');
      if (c) c.click();
      return 'closed';
    })()
  `);
  ab.waitMs(800);

  // T25.4 - 清除 → 结果清空
  ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if ((btns[i].textContent || '').trim() === '清除') { btns[i].click(); return 'cleared'; }
      }
      return 'no-clear-btn';
    })()
  `);
  ab.waitMs(1000);
  const rowsAfterClear = listRowCount();
  step('T25.4: 清除后结果清空', Number(rowsAfterClear) === 0, 'rows=' + rowsAfterClear);

  // T25.5 - 无匹配词 → 空态
  triggerSearch('zzz不存在的词xyz');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  assertCondition(ab.pageContainsText('没有匹配的节点'), 'T25.5: 无匹配显示空态', '');
  ab.screenshot('t25-04-empty-state');

  ab.closeBrowser();
});
