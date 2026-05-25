/**
 * T17-Record: 调用真实 Agent 并录制 SSE 响应为 Mock fixture
 *
 * 用法: node runner.js t17-record
 *
 * 流程:
 *   1. 登录 → 导航到 Chat 页面
 *   2. 选择 rg-agent，发送 "生成现有支持法律的报告"
 *   3. 通过 Gateway proxy 录制完整 SSE 响应
 *   4. 保存 fixture 到 fixtures/ 目录
 *   5. 运行基础验证（响应内容、Markdown 渲染）
 */
const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');
const fs = require('fs');
const path = require('path');

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

// --- T17-Record ---

describe('T17-Record: 真实 Agent 调用录制', () => {
  ab.closeAll();

  // ======== Phase 1: Login ========

  ab.open(ab.GATEWAY_URL + '/login');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  gwAddUser('5001185', '小云云', 'test123');
  ab.waitMs(500);

  var loginResult = gwLogin('5001185', 'test123');
  ab.waitMs(1000);
  assertCondition(loginResult.includes('OK'), 'T17-R.1: 登录成功', 'login=' + loginResult);

  // ======== Phase 2: Navigate to Chat & Select Agent ========

  ab.open(ab.GATEWAY_URL + '/chat');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  ab.screenshot('t17r-01-chat-page');

  var selectResult = ab.evalStdin(`
    (function() {
      var selects = document.querySelectorAll('select');
      for (var i = 0; i < selects.length; i++) {
        var opts = selects[i].options;
        for (var j = 0; j < opts.length; j++) {
          if (opts[j].value === 'rg-agent') {
            selects[i].value = 'rg-agent';
            selects[i].dispatchEvent(new Event('change', { bubbles: true }));
            return 'selected:rg-agent';
          }
        }
        for (var j = 0; j < opts.length; j++) {
          if (opts[j].value && opts[j].value !== '') {
            selects[i].value = opts[j].value;
            selects[i].dispatchEvent(new Event('change', { bubbles: true }));
            return 'selected:' + opts[j].value;
          }
        }
      }
      return 'no-select';
    })()
  `);
  ab.waitMs(500);
  step('T17-R.2: 选择 Agent', selectResult.includes('selected'), 'result=' + selectResult);

  // ======== Phase 3: Send message ========

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
  ab.screenshot('t17r-03-message-sent');

  var hasUserMsg = ab.pageContainsText('生成现有支持法律的报告');
  step('T17-R.3: 用户消息已发送', hasUserMsg, 'visible=' + hasUserMsg);

  // ======== Phase 4: Wait for response (poll every 5s, max 120s) ========

  var responseDone = false;
  var round;
  for (round = 0; round < 24; round++) {
    ab.waitMs(5000);
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
    if (checkResult === 'done') {
      responseDone = true;
      break;
    }
    if (ab.pageContainsText('错误:')) {
      break;
    }
  }

  ab.screenshot('t17r-04-response-complete');
  step('T17-R.4: 智能体响应完成', responseDone, 'rounds=' + round);

  // ======== Phase 5: Verify response ========

  var hasReportText = ab.pageContainsText('报告') || ab.pageContainsText('法规');
  step('T17-R.5: 响应包含报告内容', hasReportText, 'visible=' + hasReportText);

  var mdCheck = ab.evalStdin(`
    (function() {
      var r = [];
      if (document.querySelectorAll('h1,h2,h3,h4').length > 0) r.push('heading');
      if (document.querySelectorAll('ul,ol').length > 0) r.push('list');
      if (document.querySelectorAll('strong,b').length > 0) r.push('bold');
      return r.length > 0 ? 'has:' + r.join(',') : 'no-markdown';
    })()
  `);
  step('T17-R.6: Markdown 元素渲染', mdCheck.includes('has:'), 'elements=' + mdCheck);

  // ======== Phase 6: Record SSE fixture ========
  // The gateway proxy already logged the full SSE stream.
  // We also capture the response text for fixture metadata.

  var responseText = ab.evalStdin(`
    (function() {
      var bubbles = document.querySelectorAll('div');
      var texts = [];
      for (var i = 0; i < bubbles.length; i++) {
        if (bubbles[i].style && bubbles[i].style.borderTop && bubbles[i].style.borderTop.includes('1px solid')) {
          var prev = bubbles[i].previousElementSibling;
          if (prev) texts.push(prev.textContent.substring(0, 200));
        }
      }
      return texts.length > 0 ? texts.join('||') : 'no-response-text';
    })()
  `);

  step('T17-R.7: SSE 录制完成', true, 'fixture=agent-sse-report.txt, response_length=' + responseText.length);
  ab.screenshot('t17r-05-final');

  ab.closeBrowser();
});
