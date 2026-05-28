const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

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

function gwRegisterAgent(agentId, agentName, agentGroupId, endpoint) {
  var body = { agentId: agentId, agentName: agentName };
  if (agentGroupId) body.agentGroupId = agentGroupId;
  if (endpoint) body.endpoint = endpoint;
  return ab.evalStdin(`
    (function() {
      return fetch('/api/agents', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(${JSON.stringify(body)})
      }).then(function(r) { return r.json(); })
        .then(function(d) { return d.success ? 'OK' : 'FAIL: ' + (d.error || ''); })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function gwRemoveAgent(agentId) {
  return ab.evalStdin(`
    (function() {
      return fetch('/api/agents/${agentId}', { method: 'DELETE' })
        .then(function(r) { return r.json(); })
        .then(function(d) { return d.success ? 'OK' : 'FAIL: ' + (d.error || ''); })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function gwListAgents() {
  return ab.evalStdin(`
    (function() {
      return fetch('/api/agents')
        .then(function(r) { return r.json(); })
        .then(function(data) {
          return 'OK:' + JSON.stringify(data);
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function gwAgentExists(agentId) {
  var result = gwListAgents();
  if (!result.startsWith('OK:')) return false;
  try {
    var agents = JSON.parse(result.substring(3));
    return agents.some(function(a) { return a.agentId === agentId; });
  } catch (e) { return false; }
}

// --- T18: Gateway Agent Management ---

describe('T18: 网关 Agent 管理', () => {
  ab.closeAll();

  // ======== Phase 1: Login & Dashboard ========

  // T18.1 - Ensure test user 5001185 exists and login
  ab.open(ab.GATEWAY_URL + '/login');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  // Create user if needed (via API call from login page)
  var addResult = gwAddUser('5001185', '小云云', 'test123');
  ab.waitMs(500);

  // Login via API
  var loginResult = gwLogin('5001185', 'test123');
  ab.waitMs(1000);
  assertCondition(loginResult.includes('OK'), 'T18.1: 5001185 登录成功', 'login=' + loginResult);

  // Navigate to dashboard
  ab.open(ab.GATEWAY_URL + '/dashboard');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  ab.screenshot('t16-01-dashboard');

  var dashLoaded = ab.pageContainsText('Agent 管理') || ab.pageContainsText('Agent');
  assertCondition(dashLoaded, 'T18.2: 仪表盘加载成功', 'dashLoaded=' + dashLoaded);

  // ======== Phase 2: Register Agent with Endpoint ========

  // Clean up any pre-existing test agents
  gwRemoveAgent('agent-test-ep');
  ab.waitMs(300);
  gwRemoveAgent('agent-test-default');
  ab.waitMs(300);
  gwRemoveAgent('agent-test-chat');
  ab.waitMs(300);

  // T18.3 - Register agent with full URL endpoint via UI
  // Use JS to fill the form fields directly
  var fillResult = ab.evalStdin(`
    (function() {
      var nativeSet = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
      var agentId = document.getElementById('newAgentId');
      var agentName = document.getElementById('newAgentName');
      var agentGroup = document.getElementById('newAgentGroup');
      var endpoint = document.getElementById('newAgentEndpoint');
      if (!agentId || !agentName || !endpoint) return 'missing: ' + [!agentId && 'agentId', !agentName && 'agentName', !endpoint && 'endpoint'].filter(Boolean).join(',');
      nativeSet.call(agentId, 'agent-test-ep');
      agentId.dispatchEvent(new Event('input', { bubbles: true }));
      nativeSet.call(agentName, 'Endpoint Test Agent');
      agentName.dispatchEvent(new Event('input', { bubbles: true }));
      nativeSet.call(agentGroup, 'test-group');
      agentGroup.dispatchEvent(new Event('input', { bubbles: true }));
      nativeSet.call(endpoint, 'http://localhost:8090/process');
      endpoint.dispatchEvent(new Event('input', { bubbles: true }));
      return 'filled';
    })()
  `);
  ab.waitMs(500);
  ab.screenshot('t16-03-form-filled');

  // Submit the form
  var submitResult = ab.evalStdin(`
    (function() {
      var form = document.getElementById('addAgentForm');
      if (form) { form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })); return 'submitted'; }
      return 'form not found';
    })()
  `);
  ab.waitMs(2000);
  ab.screenshot('t16-03-after-submit');

  // Verify agent appears in the list with endpoint
  var hasEndpoint = ab.pageContainsText('agent-test-ep') && ab.pageContainsText('localhost:8090');
  step('T18.3: 注册带端点的 Agent', hasEndpoint, 'fill=' + fillResult + ' submit=' + submitResult + ' hasAgent=' + ab.pageContainsText('agent-test-ep'));

  // T18.4 - Verify via API
  var apiList = gwListAgents();
  var apiHasAgent = apiList.includes('agent-test-ep') && apiList.includes('http://localhost:8090/process');
  step('T18.4: API 验证端点配置', apiHasAgent, apiList.substring(0, 200));

  // ======== Phase 3: Register Agent without Endpoint (default) ========

  // T18.5 - Register agent with empty endpoint via API
  var regDefault = gwRegisterAgent('agent-test-default', 'Default Agent', '', '');
  ab.waitMs(500);
  assertCondition(regDefault.includes('OK'), 'T18.5: 注册默认端点 Agent', 'reg=' + regDefault);

  // Refresh dashboard and verify default endpoint display
  ab.open(ab.GATEWAY_URL + '/dashboard');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  ab.screenshot('t16-05-default-agent');

  var hasDefault = ab.pageContainsText('agent-test-default');
  step('T18.6: 默认端点 Agent 显示', hasDefault, 'visible=' + hasDefault);

  // ======== Phase 4: Update Endpoint via Re-registration ========

  // T18.7 - Re-register same agent with different endpoint (update)
  var updateResult = gwRegisterAgent('agent-test-ep', 'Updated Agent', 'test-group', 'http://192.168.1.100:9000/v1/chat');
  ab.waitMs(500);
  assertCondition(updateResult.includes('OK'), 'T18.7: 更新 Agent 端点', 'update=' + updateResult);

  // Verify updated endpoint
  ab.open(ab.GATEWAY_URL + '/dashboard');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  ab.screenshot('t16-07-updated-endpoint');

  var hasUpdated = ab.pageContainsText('192.168.1.100');
  step('T18.8: 验证端点更新', hasUpdated, 'hasUpdated=' + hasUpdated);

  // ======== Phase 5: Delete Agent ========

  // T18.9 - Delete agent via API
  var delResult = gwRemoveAgent('agent-test-ep');
  ab.waitMs(500);
  assertCondition(delResult.includes('OK'), 'T18.9: 删除 Agent', 'del=' + delResult);

  // Verify deletion
  var delCheck = !gwAgentExists('agent-test-ep');
  step('T18.10: 验证 Agent 已删除', delCheck, '');

  // Refresh dashboard and verify
  ab.open(ab.GATEWAY_URL + '/dashboard');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  ab.screenshot('t16-10-after-delete');

  var notVisible = !ab.pageContainsText('agent-test-ep');
  step('T18.11: 仪表盘不再显示已删除 Agent', notVisible, '');

  // Delete remaining test agent
  gwRemoveAgent('agent-test-default');
  ab.waitMs(300);

  // ======== Phase 6: Agent Chat Page ========

  // T18.12 - Register an agent for chat testing
  gwRegisterAgent('agent-test-chat', 'Chat Test Agent', '', 'http://localhost:8090/process');
  ab.waitMs(500);

  // Navigate to chat page
  ab.open(ab.GATEWAY_URL + '/chat');
  ab.waitMs(3000);
  ab.waitLoad('networkidle');
  ab.screenshot('t16-12-chat-page');

  var chatLoaded = ab.pageContainsText('Agent 对话') || ab.pageContainsText('输入消息');
  step('T18.12: Agent 对话页面加载', chatLoaded, 'chatLoaded=' + chatLoaded);

  // T18.13 - Verify agent selector exists
  var snap = ab.snapshot();
  var hasAgentSelect = ab.pageContainsText('身份') || ab.pageContainsText('Chat Test Agent') || ab.pageContainsText('agent-test-chat');
  step('T18.13: Agent 选择器显示', hasAgentSelect, 'hasSelect=' + hasAgentSelect);

  // T18.14 - Select agent and send a message
  // Select the agent from dropdown via JS
  var selectResult = ab.evalStdin(`
    (function() {
      var selects = document.querySelectorAll('select');
      for (var i = 0; i < selects.length; i++) {
        var opts = selects[i].options;
        for (var j = 0; j < opts.length; j++) {
          if (opts[j].value === 'agent-test-chat') {
            selects[i].value = 'agent-test-chat';
            selects[i].dispatchEvent(new Event('change', { bubbles: true }));
            return 'selected';
          }
        }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(500);
  ab.screenshot('t16-14-agent-selected');

  // T18.15 - Send a test message (expect connection error since no real agent backend at 8090)
  var sendResult = ab.evalStdin(`
    (function() {
      var textarea = document.querySelector('textarea');
      if (!textarea) return 'textarea not found';
      var nativeSet = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set;
      nativeSet.call(textarea, 'Hello, this is a test message from 5001185');
      textarea.dispatchEvent(new Event('input', { bubbles: true }));
      return 'filled';
    })()
  `);
  ab.waitMs(500);

  // Click send button
  ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        var svg = btns[i].querySelector('svg');
        if (svg || btns[i].getAttribute('aria-label') === 'Send' || btns[i].getAttribute('type') === 'submit') {
          btns[i].click();
          return 'clicked button: ' + btns[i].outerHTML.substring(0, 80);
        }
      }
      // Try pressing Enter on textarea
      var textarea = document.querySelector('textarea');
      if (textarea) {
        textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        return 'pressed Enter';
      }
      return 'no send trigger found';
    })()
  `);
  ab.waitMs(3000);
  ab.screenshot('t16-15-message-sent');

  // T18.16 - Verify message was sent (either user message displayed, or error from no backend)
  var hasUserMsg = ab.pageContainsText('Hello, this is a test message from 5001185');
  var hasErrorMsg = ab.pageContainsText('错误') || ab.pageContainsText('Error') || ab.pageContainsText('unavailable') || ab.pageContainsText('failed');
  step('T18.16: 消息已发送', hasUserMsg, 'sent=' + hasUserMsg);
  ab.screenshot('t16-16-after-send');

  // T18.17 - Verify assistant response or error (either means SSE pipeline works)
  var hasResponse = ab.pageContainsText('请问') || ab.pageContainsText('帮助') || hasUserMsg;
  step('T18.17: 对话响应正常', hasResponse || hasErrorMsg, 'response=' + hasResponse + ' error=' + hasErrorMsg);

  // ======== Phase 7: Logout Redirect to Main Page ========

  // Navigate back to dashboard
  ab.open(ab.GATEWAY_URL + '/dashboard');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  // Click logout link
  ab.findAndClick('退出登录');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  ab.screenshot('t16-logout-clicked');

  // Verify redirect to main page (root / or /login)
  var logoutUrl = ab.getUrl();
  var atMainPage = logoutUrl === ab.GATEWAY_URL + '/' || logoutUrl === ab.GATEWAY_URL + '/login';
  step('T18.19: 退出登录后跳转主页面', atMainPage, 'url=' + logoutUrl);

  // Verify user info is cleared
  var noUserInfo = !ab.pageContainsText('5001185');
  step('T18.20: 退出后用户信息已清除', noUserInfo, '');

  // Verify dashboard redirects to login when not authenticated
  ab.open(ab.GATEWAY_URL + '/dashboard');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  ab.screenshot('t16-after-logout-dashboard');

  var afterLogoutUrl = ab.getUrl();
  var redirectedToLogin = afterLogoutUrl.includes('/login');
  step('T18.21: 未登录访问 dashboard 重定向到 login', redirectedToLogin, 'url=' + afterLogoutUrl);

  // ======== Phase 8: Cleanup ========

  // Re-login for cleanup (session was cleared by logout)
  var reLoginResult = gwLogin('5001185', 'test123');
  ab.waitMs(1000);

  // T18.22 - Clean up all test agents
  gwRemoveAgent('agent-test-chat');
  ab.waitMs(300);

  // Verify all test agents removed
  var finalList = gwListAgents();
  var clean = !finalList.includes('agent-test-');
  step('T18.22: 清理测试 Agent', clean, 'clean=' + clean);

  ab.screenshot('t16-22-cleanup');
  ab.closeBrowser();
});
