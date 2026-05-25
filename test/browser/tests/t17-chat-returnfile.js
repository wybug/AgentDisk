/**
 * T17: Chat ReturnFile + Markdown 测试 (Mock 模式)
 *
 * 使用 Mock Agent Server 快速验证 UI 功能，无需真实 Agent。
 * 前置条件:
 *   - fixture 文件存在于 fixtures/ 目录
 *   - mock-agent-server.js 提供回放服务
 *
 * 用法: node runner.js t17
 *
 * 如需录制真实数据: node runner.js t17-record
 */
const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');
const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

const MOCK_PORT = 9876;
const FIXTURE_DIR = path.join(__dirname, '..', 'fixtures');

// --- Helpers ---

function gwLogin(userId, password) {
  return ab.evalStdin(`
    (function() {
      return fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ userId: '${userId}', password: '${password}' })
      }).then(function(r) { return r.json(); })
        .then(function(d) { return d.success ? 'OK' : 'FAIL: ' + (d.message || ''); })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function gwAddUser(userId, userName, password) {
  return ab.evalStdin(`
    (function() {
      return fetch('/api/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ userId: '${userId}', userName: '${userName}', password: '${password}' })
      }).then(function(r) { return r.json(); })
        .then(function(d) { return d.success ? 'OK' : 'FAIL: ' + (d.error || ''); })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function gwRegisterAgent(agentId, agentName, endpoint) {
  return ab.evalStdin(`
    (function() {
      return fetch('/api/agents', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agentId: '${agentId}', agentName: '${agentName}', endpoint: '${endpoint}' })
      }).then(function(r) { return r.json(); })
        .then(function(d) { return d.success ? 'OK' : 'FAIL: ' + (d.error || ''); })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

// --- Check fixtures exist ---
var reportFixture = path.join(FIXTURE_DIR, 'agent-sse-report.txt');
var simpleFixture = path.join(FIXTURE_DIR, 'agent-sse-simple.txt');

if (!fs.existsSync(reportFixture) || !fs.existsSync(simpleFixture)) {
  console.log('\n\x1b[33m⚠ Mock fixture 不存在，请先运行录制:\x1b[0m');
  console.log('  \x1b[36mnode runner.js t17-record\x1b[0m\n');
  process.exit(0);
}

// --- T17: Mock Mode Test ---

describe('T17: Chat ReturnFile + Markdown 测试 (Mock)', () => {
  ab.closeAll();

  // ======== Phase 0: Start mock agent server ========

  var mockServer = spawn('node', [
    path.join(FIXTURE_DIR, 'mock-agent-server.js'),
    String(MOCK_PORT),
    FIXTURE_DIR,
  ], { stdio: ['pipe', 'pipe', 'pipe'], detached: true });
  mockServer.unref();

  // Wait for mock server to be ready
  var mockReady = false;
  for (var w = 0; w < 10; w++) {
    ab.waitMs(500);
    try {
      var { execSync } = require('child_process');
      execSync('curl -s -o /dev/null -w "%{http_code}" http://localhost:' + MOCK_PORT + '/ -X POST -H "Content-Type: application/json" -d \'{}\'', { timeout: 2000 });
      mockReady = true;
      break;
    } catch {}
  }
  assertCondition(mockReady, 'T17.0: Mock Agent Server 启动', 'port=' + MOCK_PORT);

  // ======== Phase 1: Login ========

  ab.open(ab.GATEWAY_URL + '/login');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  gwAddUser('5001185', '小云云', 'test123');
  ab.waitMs(500);

  var loginResult = gwLogin('5001185', 'test123');
  ab.waitMs(1000);
  assertCondition(loginResult.includes('OK'), 'T17.1: 登录成功', 'login=' + loginResult);

  // ======== Phase 2: Register mock agent & Navigate ========

  var registerResult = gwRegisterAgent('mock-agent', 'Mock测试Agent', 'http://localhost:' + MOCK_PORT + '/process');
  ab.waitMs(300);
  step('T17.2: 注册 Mock Agent', registerResult.includes('OK'), 'result=' + registerResult);

  ab.open(ab.GATEWAY_URL + '/chat');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  ab.screenshot('t17-01-chat-page');

  var chatLoaded = ab.pageContainsText('Agent 对话') || ab.pageContainsText('输入消息');
  step('T17.3: 对话页面加载', chatLoaded, 'chatLoaded=' + chatLoaded);

  // ======== Phase 3: Override fetch to inject mock-agent ID ========
  // React controlled select doesn't reliably update state via native events.
  // Instead, override fetch to ensure agentId is always set.

  var fetchOverride = ab.evalStdin(`
    (function() {
      var orig = window.fetch;
      window.fetch = function(url, opts) {
        if (typeof url === 'string' && url === '/process' && opts && opts.body) {
          try {
            var body = JSON.parse(opts.body);
            if (!body.agentId) {
              body.agentId = 'mock-agent';
              opts = Object.assign({}, opts, { body: JSON.stringify(body) });
            }
          } catch(e) {}
        }
        return orig.apply(this, arguments);
      };
      return 'ok';
    })()
  `);
  step('T17.3b: Fetch agentId 注入', fetchOverride.includes('ok'), 'result=' + fetchOverride);

  // Also select in dropdown for visual consistency
  ab.evalStdin(`
    (function() {
      var selects = document.querySelectorAll('select');
      for (var i = 0; i < selects.length; i++) {
        var opts = selects[i].options;
        for (var j = 0; j < opts.length; j++) {
          if (opts[j].value === 'mock-agent') {
            selects[i].value = 'mock-agent';
            selects[i].dispatchEvent(new Event('change', { bubbles: true }));
            return 'selected:mock-agent';
          }
        }
      }
      return 'no-select';
    })()
  `);
  ab.waitMs(1000);
  ab.screenshot('t17-03-agent-selected');

  var hasSelect = ab.evalStdin(`
    (function() {
      var sel = document.querySelector('select');
      return sel && sel.value ? 'value:' + sel.value : 'empty';
    })()
  `);
  step('T17.4: Agent 选择状态', hasSelect.includes('mock-agent') || hasSelect !== 'empty', 'select=' + hasSelect);

  // ======== Phase 4: Send message (report prompt) ========

  ab.evalStdin(`
    (function() {
      var textarea = document.querySelector('textarea');
      if (!textarea) return 'no-textarea';
      var nativeSet = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set;
      nativeSet.call(textarea, '生成现有支持法律的报告');
      textarea.dispatchEvent(new Event('input', { bubbles: true }));
      return 'filled';
    })()
  `);
  ab.waitMs(300);

  ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        var svg = btns[i].querySelector('svg');
        if (svg || btns[i].getAttribute('type') === 'submit') {
          btns[i].click();
          return 'clicked';
        }
      }
      var textarea = document.querySelector('textarea');
      if (textarea) {
        textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        return 'enter';
      }
      return 'no-send';
    })()
  `);
  ab.waitMs(2000);
  ab.screenshot('t17-05-message-sent');

  var hasUserMsg = ab.pageContainsText('生成现有支持法律的报告');
  step('T17.5: 用户消息已发送', hasUserMsg, 'visible=' + hasUserMsg);

  // ======== Phase 5: Wait for mock response (max 30s, should be fast) ========

  var responseDone = false;
  for (var round = 0; round < 15; round++) {
    ab.waitMs(2000);
    var checkResult = ab.evalStdin(`
      (function() {
        var stopBtns = document.querySelectorAll('button');
        for (var i = 0; i < stopBtns.length; i++) {
          var text = stopBtns[i].textContent || '';
          if (text.includes('Stop') || text.includes('停止')) return 'streaming';
        }
        return 'done';
      })()
    `);
    if (checkResult.includes('done')) {
      responseDone = true;
      break;
    }
    if (ab.pageContainsText('错误:')) {
      break;
    }
  }

  ab.screenshot('t17-06-response-complete');
  step('T17.6: Mock 响应完成', responseDone, 'rounds=' + round);

  // ======== Phase 6: Verify response content ========

  var hasReportText = ab.pageContainsText('报告') || ab.pageContainsText('法规');
  step('T17.7: 响应包含报告内容', hasReportText, 'visible=' + hasReportText);

  // Check Markdown elements
  var mdCheck = ab.evalStdin(`
    (function() {
      var r = [];
      if (document.querySelectorAll('h1,h2,h3,h4').length > 0) r.push('heading');
      if (document.querySelectorAll('ul,ol').length > 0) r.push('list');
      if (document.querySelectorAll('table').length > 0) r.push('table');
      if (document.querySelectorAll('strong,b').length > 0) r.push('bold');
      if (document.querySelectorAll('pre code').length > 0) r.push('code');
      return r.length > 0 ? 'has:' + r.join(',') : 'no-markdown';
    })()
  `);
  step('T17.8: Markdown 元素渲染', mdCheck.includes('has:'), 'elements=' + mdCheck);
  ab.screenshot('t17-07-markdown');

  // ======== Phase 7: Verify Metrics popover ========

  // Debug: dump all divs with borderTop to understand DOM structure
  var footerDebug = ab.evalStdin(`
    (function() {
      var allDivs = document.querySelectorAll('div');
      var results = [];
      for (var i = 0; i < allDivs.length; i++) {
        var s = allDivs[i].style;
        if (s && s.borderTop && s.borderTop.includes('1px solid')) {
          var svgs = allDivs[i].querySelectorAll('svg');
          var tag = allDivs[i].className || '';
          var childTags = [];
          for (var c = 0; c < allDivs[i].children.length; c++) {
            var ch = allDivs[i].children[c];
            childTags.push(ch.tagName + (ch.style && ch.style.cursor === 'pointer' ? '[ptr]' : ''));
          }
          results.push('svg=' + svgs.length + ' children=[' + childTags.join(',') + ']');
        }
      }
      return results.length > 0 ? results.join('||') : 'no-borderTop-div';
    })()
  `);
  step('T17.9-debug: Footer DOM 结构', true, 'info=' + footerDebug);

  // Click the last SVG in the footer (BarChart3 is always the last action icon)
  var metricsResult = ab.evalStdin(`
    (function() {
      var allDivs = document.querySelectorAll('div');
      for (var i = 0; i < allDivs.length; i++) {
        var s = allDivs[i].style;
        if (s && s.borderTop && s.borderTop.includes('1px solid')) {
          var svgs = allDivs[i].querySelectorAll('svg');
          if (svgs.length > 0) {
            var last = svgs[svgs.length - 1];
            last.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
            return 'clicked-last-svg total=' + svgs.length;
          }
        }
      }
      return 'no-footer-svgs';
    })()
  `);
  ab.waitMs(500);
  ab.screenshot('t17-08-metrics-click');

  var popoverText = ab.evalStdin(`
    (function() {
      var divs = document.querySelectorAll('div');
      for (var i = 0; i < divs.length; i++) {
        var s = divs[i].style;
        if (s && s.position === 'fixed' && s.boxShadow && s.boxShadow !== '') {
          var t = divs[i].textContent || '';
          if (t.includes('Token') || t.includes('耗时')) return t.substring(0, 150);
        }
      }
      return 'no-popover';
    })()
  `);
  step('T17.9: Metrics 弹窗显示', popoverText.includes('Token'), 'text=' + popoverText + ', click=' + metricsResult);

  // ======== Phase 8: Verify Markdown toggle ========

  var toggleResult = ab.evalStdin(`
    (function() {
      var footers = document.querySelectorAll('div');
      for (var i = 0; i < footers.length; i++) {
        var s = footers[i].style;
        if (s && s.borderTop && s.borderTop.includes('1px solid')) {
          var spans = footers[i].querySelectorAll('span');
          for (var j = 0; j < spans.length; j++) {
            if (spans[j].style && spans[j].style.cursor === 'pointer') {
              var svg = spans[j].querySelector('svg');
              if (svg) { spans[j].click(); return 'toggled'; }
            }
          }
        }
      }
      return 'no-toggle';
    })()
  `);
  ab.waitMs(300);
  ab.screenshot('t17-09-toggle');
  step('T17.10: Markdown 切换按钮', toggleResult.includes('toggled'), 'result=' + toggleResult);

  if (toggleResult.includes('toggled')) {
    var hasPre = ab.evalStdin(`
      (function() {
        var pres = document.querySelectorAll('pre');
        for (var i = 0; i < pres.length; i++) {
          if ((pres[i].textContent || '').length > 50) return 'raw-visible';
        }
        return 'no-raw';
      })()
    `);
    step('T17.11: 原始文本显示', hasPre.includes('raw-visible'), 'check=' + hasPre);
  }

  // ======== Phase 9: Test simple response ========

  // Type and send a simple message via form submit approach
  var fillResult = ab.evalStdin(`
    (function() {
      var textarea = document.querySelector('textarea');
      if (!textarea) return 'no-textarea';
      var nativeSet = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set;
      nativeSet.call(textarea, '你好');
      textarea.dispatchEvent(new Event('input', { bubbles: true }));
      return 'value:' + textarea.value;
    })()
  `);
  ab.waitMs(500);

  // Click send: look for button with svg that is NOT a Stop button
  var sendResult = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        var txt = (btns[i].textContent || '').trim();
        if (txt.includes('Stop') || txt.includes('停止')) continue;
        if (btns[i].getAttribute('type') === 'submit') {
          btns[i].click();
          return 'submit-clicked';
        }
      }
      for (var i = 0; i < btns.length; i++) {
        var txt = (btns[i].textContent || '').trim();
        if (txt.includes('Stop') || txt.includes('停止')) continue;
        var svg = btns[i].querySelector('svg');
        if (svg) {
          btns[i].click();
          return 'svg-clicked:' + i;
        }
      }
      return 'no-send';
    })()
  `);
  ab.waitMs(5000);
  ab.screenshot('t17-10-simple-response');

  var hasHello = ab.pageContainsText('智能体') || ab.pageContainsText('服务');
  step('T17.12: 简单对话响应', hasHello, 'visible=' + hasHello + ', fill=' + fillResult + ', send=' + sendResult);

  // ======== Cleanup ========

  // Stop mock server
  try { process.kill(-mockServer.pid); } catch {}

  ab.screenshot('t17-final');
  ab.closeBrowser();
});
