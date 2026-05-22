const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T12: 测试网关管理', () => {
  ab.closeAll();

  // Login to gateway first
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

  ab.open(ab.GATEWAY_URL + '/login');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  var loginResult = gwLogin('user001', 'test123');
  ab.waitMs(1000);

  // T12.1 - 访问网关仪表盘
  ab.open(ab.GATEWAY_URL + '/dashboard');
  ab.waitMs(3000);
  ab.waitLoad('networkidle');
  ab.screenshot('t12-01-dashboard');

  const dashLoaded = ab.pageContainsText('AgentDisk') || ab.pageContainsText('网关') || ab.pageContainsText('OAuth2');
  assertCondition(dashLoaded, 'T12.1: 网关仪表盘加载成功');

  // T12.2 - 查看 OAuth2 配置区域
  const hasOAuth = ab.pageContainsText('Client') || ab.pageContainsText('OAuth2') || ab.pageContainsText('oauth');
  assertCondition(hasOAuth, 'T12.2: OAuth2 配置信息显示');
  ab.screenshot('t12-02-oauth-config');

  // T12.3 - 添加测试用户（使用直接 ID 选择器避免 placeholder 匹配冲突）
  const addUserResult = ab.evalStdin(`
    (function() {
      var userIdInput = document.getElementById('newUserId');
      var userNameInput = document.getElementById('newUserName');
      var passwordInput = document.getElementById('newPassword');
      if (!userIdInput) return 'no userId input';
      var nativeSet = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
      nativeSet.call(userIdInput, 'testuser999');
      userIdInput.dispatchEvent(new Event('input', { bubbles: true }));
      if (userNameInput) {
        nativeSet.call(userNameInput, '测试用户');
        userNameInput.dispatchEvent(new Event('input', { bubbles: true }));
      }
      if (passwordInput) {
        nativeSet.call(passwordInput, 'test123');
        passwordInput.dispatchEvent(new Event('input', { bubbles: true }));
      }
      return 'filled';
    })()
  `);
  ab.waitMs(500);

  // 提交表单（直接 submit 触发 form 事件）
  const addClick = ab.evalStdin(`
    (function() {
      var form = document.getElementById('addUserForm');
      if (form) { form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })); return 'submitted'; }
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('添加')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(2000);

  const added = ab.pageContainsText('testuser999');
  step('T12.3: 测试用户添加', added, 'fill=' + addUserResult + ' click=' + addClick);
  ab.screenshot('t12-03-user-added');

  // T12.4 - 通过 API 删除测试用户（绕过 confirm() 阻塞问题）
  if (added) {
    const delResult = ab.evalStdin(`
      (function() {
        return fetch('/api/users/testuser999', { method: 'DELETE' })
          .then(function(r) { return r.json(); })
          .then(function(d) { return 'OK: deleted'; })
          .catch(function(e) { return 'ERR: ' + e.message; });
      })()
    `);
    ab.waitMs(2000);
    ab.evalStdin('loadUsers()'); // trigger table refresh
    ab.waitMs(1000);
    const gone = !ab.pageContainsText('testuser999');
    step('T12.4: 测试用户删除', gone, delResult);
  } else {
    step('T12.4: 跳过删除（用户未添加成功）', false, '前置条件不满足');
  }
  ab.screenshot('t12-04-user-deleted');

  // T12.5 - 验证「打开 AgentDisk」链接
  const hasOpenLink = ab.pageContainsText('AgentDisk') || ab.pageContainsText('9101');
  step('T12.5: 页面包含 AgentDisk 链接信息', hasOpenLink);
  ab.screenshot('t12-05-agentdisk-link');

  // T12.6 - 访问 OAuth2 调试页面
  ab.open(ab.GATEWAY_URL + '/oauth2/debug');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  const debugLoaded = ab.pageContainsText('OAuth2') || ab.pageContainsText('debug') || ab.pageContainsText('授权');
  step('T12.6: OAuth2 调试页面加载', debugLoaded);
  ab.screenshot('t12-06-debug-page');

  ab.closeBrowser();
});
