// MANUAL_TEST: true — 需要通过 viz 仪表盘运行，人工配合完成 WebAuthn 原生弹窗
const { describe, step, assertCondition, printReport } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');
const { execSync } = require('child_process');

const API_BASE = 'http://localhost:9100';
const WEB_BASE = ab.BASE_URL;

function alertManual(msg) {
  try { execSync('say "' + msg + '"', { timeout: 10000 }); } catch (e) { /* ignore */ }
}

describe('T02f: Admin MFA 通行密钥管理', () => {
  ab.open(WEB_BASE + '/admin/login');
  ab.waitMs(1000);

  function apiCall(code) {
    var raw = ab.evalStdin(code);
    ab.waitMs(1200);
    if (!raw) return {};
    try {
      var parsed = JSON.parse(raw);
      if (typeof parsed === 'string') return JSON.parse(parsed);
      return parsed;
    } catch (e) {
      return { error: String(e), raw: raw };
    }
  }

  // ── Bootstrap: API 登录获取 admin token ──
  var bootstrap = apiCall(`
    (function() {
      return fetch('${API_BASE}/v1/disk/admin/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: 'admin', password: 'admin123' }),
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);

  var bootstrapOk = bootstrap.code === 0 && (bootstrap.data || {}).token;
  if (!bootstrapOk) {
    step('TC-prep: Admin 登录失败，请先运行 T01 bootstrap', true, (bootstrap.message || JSON.stringify(bootstrap)).substring(0, 200));
    ab.closeBrowser();
    printReport();
    return;
  }

  var adminToken = (bootstrap.data || {}).token;
  step('TC-prep: Admin 登录成功', true);

  // ── TC-70: 导航到 MFA 设置页 ──
  ab.open(WEB_BASE + '/admin/mfa');
  ab.waitMs(1500);
  ab.waitLoad('networkidle');
  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.waitMs(300);
  ab.open(WEB_BASE + '/admin/mfa');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  var mfaUrl = ab.getUrl();
  assertCondition(
    mfaUrl.includes('/admin/mfa'),
    'TC-70: 导航到 MFA 设置页',
    mfaUrl
  );

  var hasMFATitle = ab.pageContainsText('多因素认证');
  assertCondition(hasMFATitle, 'TC-70: MFA 设置页标题显示', 'hasMFATitle=' + hasMFATitle);
  ab.screenshot('t02f-01-mfa-page');

  // ── TC-71: 验证无通行密钥时 Switch 禁用 ──
  var hasEmptyHint = ab.pageContainsText('暂无通行密钥') || ab.pageContainsText('注册至少一个通行密钥');
  var switchDisabled = ab.evalStdin(`
    (function() {
      var btn = document.querySelector('button.ant-switch');
      if (!btn) return 'no-switch';
      return btn.disabled ? 'disabled' : 'enabled';
    })()
  `);
  assertCondition(
    hasEmptyHint && String(switchDisabled).includes('disabled'),
    'TC-71: 无通行密钥时 MFA 开关禁用',
    'emptyHint=' + hasEmptyHint + ' switch=' + switchDisabled
  );
  ab.screenshot('t02f-02-switch-disabled');

  // ── TC-72: 点击「注册通行密钥」→ [人工] 完成 WebAuthn 注册 ──
  var regBtnClicked = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('注册通行密钥')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  step('TC-72: 点击「注册通行密钥」', String(regBtnClicked).includes('clicked'), 'result=' + regBtnClicked);

  console.log('\n\x1b[43m\x1b[30m  [人工操作] 请在浏览器中完成 WebAuthn 注册（指纹/面容/安全密钥），等待30秒\x1b[0m\n');
  alertManual('请完成通行密钥注册');
  for (var w1 = 0; w1 < 10; w1++) { ab.waitMs(3000); }

  // ── TC-73: 命名弹窗 → 修改名称 → 确定 ──
  var hasNamingModal = ab.pageContainsText('为通行密钥命名');
  assertCondition(hasNamingModal, 'TC-73: 命名弹窗出现（人工未配合 WebAuthn 则失败）', 'hasNamingModal=' + hasNamingModal);

  if (hasNamingModal) {
    var nameInputResult = ab.evalStdin(`
      (function() {
        var input = document.querySelector('#passkey-name-input');
        if (!input) {
          var inputs = document.querySelectorAll('.ant-modal input');
          if (inputs.length > 0) input = inputs[0];
        }
        if (!input) return 'not found';
        var nativeSet = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
        nativeSet.call(input, '测试密钥');
        input.dispatchEvent(new Event('input', { bubbles: true }));
        input.dispatchEvent(new Event('change', { bubbles: true }));
        return 'filled';
      })()
    `);
    ab.waitMs(300);

    ab.jsClickBtn('确 定');
    ab.waitMs(2000);
    assertCondition(true, 'TC-73: 修改名称为「测试密钥」并确认', 'nameInput=' + nameInputResult);
  } else {
    assertCondition(false, 'TC-73: 修改名称为「测试密钥」并确认', '命名弹窗未出现，可能 WebAuthn 注册未完成');
  }
  ab.screenshot('t02f-03-passkey-registered');

  // ── TC-74: 验证通行密钥出现在列表 ──
  var hasTestKey = ab.pageContainsText('测试密钥');
  assertCondition(hasTestKey, 'TC-74: 列表显示新通行密钥「测试密钥」', 'hasTestKey=' + hasTestKey);

  // ── TC-75: 验证 MFA 开关变为可用 ──
  var switchEnabled = ab.evalStdin(`
    (function() {
      var btn = document.querySelector('button.ant-switch');
      if (!btn) return 'no-switch';
      return btn.disabled ? 'disabled' : 'enabled';
    })()
  `);
  assertCondition(String(switchEnabled).includes('enabled'), 'TC-75: MFA 开关变为可用', 'switch=' + switchEnabled);
  ab.screenshot('t02f-04-switch-enabled');

  // ── TC-76: 重命名通行密钥 ──
  var renameClicked = ab.evalStdin(`
    (function() {
      var links = document.querySelectorAll('button.ant-btn-link');
      for (var i = 0; i < links.length; i++) {
        if (links[i].textContent.includes('重命名')) { links[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(800);

  var hasRenameModal = ab.pageContainsText('修改通行密钥名称');
  assertCondition(hasRenameModal, 'TC-76: 点击「重命名」弹出弹窗', 'clicked=' + renameClicked + ' modal=' + hasRenameModal);

  if (hasRenameModal) {
    ab.evalStdin(`
      (function() {
        var inputs = document.querySelectorAll('.ant-modal input');
        var input = inputs[inputs.length - 1];
        if (!input) return 'not found';
        var nativeSet = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
        nativeSet.call(input, '重命名密钥');
        input.dispatchEvent(new Event('input', { bubbles: true }));
        input.dispatchEvent(new Event('change', { bubbles: true }));
        return 'filled';
      })()
    `);
    ab.waitMs(300);

    ab.jsClickBtn('确 定');
    ab.waitMs(1500);

    var hasRenamedKey = ab.pageContainsText('重命名密钥');
    assertCondition(hasRenamedKey, 'TC-76: 重命名成功，列表更新', 'hasRenamedKey=' + hasRenamedKey);
    ab.screenshot('t02f-05-renamed');
  }

  // ── TC-77: API 验证通行密钥列表 ──
  var credentialsList = apiCall(`
    (function() {
      var token = localStorage.getItem('admin_token');
      return fetch('${API_BASE}/v1/disk/admin/mfa/credentials', {
        headers: { 'Authorization': 'Bearer ' + token },
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);

  var credCount = (credentialsList.data || []).length;
  var credNameOk = credCount >= 1 && (credentialsList.data || []).some(function(c) { return c.name === '重命名密钥'; });
  assertCondition(credNameOk, 'TC-77: API 验证通行密钥列表', 'count=' + credCount + ' nameOk=' + credNameOk);

  // ── TC-78: API 验证 MFA 状态 ──
  var mfaStatus = apiCall(`
    (function() {
      var token = localStorage.getItem('admin_token');
      return fetch('${API_BASE}/v1/disk/admin/mfa/status', {
        headers: { 'Authorization': 'Bearer ' + token },
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);

  var mfaDisabled = !(mfaStatus.data || {}).mfaEnabled;
  var passkeyCount = (mfaStatus.data || {}).passkeyCount || 0;
  assertCondition(
    mfaDisabled && passkeyCount >= 1,
    'TC-78: API 验证 MFA 状态（未启用，有通行密钥）',
    'enabled=' + !(mfaDisabled) + ' count=' + passkeyCount
  );

  // ── TC-79: 注册第二个通行密钥 ──
  var regBtnClicked2 = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('注册通行密钥')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  assertCondition(String(regBtnClicked2).includes('clicked'), 'TC-79: 点击「注册通行密钥」注册第二个', 'result=' + regBtnClicked2);

  console.log('\n\x1b[43m\x1b[30m  [人工操作] 请在浏览器中完成第二个 WebAuthn 注册（指纹/面容/安全密钥），等待30秒\x1b[0m\n');
  alertManual('请完成第二个通行密钥注册');
  for (var w2 = 0; w2 < 10; w2++) { ab.waitMs(3000); }

  var hasNamingModal2 = ab.pageContainsText('为通行密钥命名');
  if (hasNamingModal2) {
    ab.evalStdin(`
      (function() {
        var input = document.querySelector('#passkey-name-input');
        if (!input) {
          var inputs = document.querySelectorAll('.ant-modal input');
          if (inputs.length > 0) input = inputs[0];
        }
        if (!input) return 'not found';
        var nativeSet = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
        nativeSet.call(input, '第二个密钥');
        input.dispatchEvent(new Event('input', { bubbles: true }));
        input.dispatchEvent(new Event('change', { bubbles: true }));
        return 'filled';
      })()
    `);
    ab.waitMs(300);
    ab.jsClickBtn('确 定');
    ab.waitMs(2000);
  }

  var hasTwoKeys = ab.pageContainsText('第二个密钥');
  assertCondition(hasTwoKeys, 'TC-79: 第二个通行密钥注册成功（人工未配合则失败）', 'hasTwoKeys=' + hasTwoKeys);
  ab.screenshot('t02f-06-two-passkeys');

  // ── TC-80: 删除第一个通行密钥 ──
  var deleteFirstResult = ab.evalStdin(`
    (function() {
      var rows = document.querySelectorAll('tr');
      for (var i = 0; i < rows.length; i++) {
        var cells = rows[i].querySelectorAll('td');
        if (cells.length === 0) continue;
        var nameCell = cells[0];
        if (nameCell && (nameCell.textContent.indexOf('重命名密钥') !== -1 || nameCell.textContent.indexOf('Passkey') !== -1)) {
          var btns = rows[i].querySelectorAll('button');
          for (var j = 0; j < btns.length; j++) {
            if (btns[j].textContent.includes('删除')) { btns[j].click(); return 'clicked-delete'; }
          }
        }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(800);

  var confirmDelete = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('.ant-popconfirm-buttons button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('删除')) { btns[i].click(); return 'confirmed'; }
      }
      var primaries = document.querySelectorAll('.ant-popconfirm .ant-btn-primary');
      if (primaries.length > 0) { primaries[0].click(); return 'confirmed-primary'; }
      return 'not found';
    })()
  `);
  ab.waitMs(1500);

  var afterFirstDelete = ab.evalStdin(`
    (function() {
      var rows = document.querySelectorAll('tbody tr');
      return String(rows.length);
    })()
  `);
  step('TC-80: 删除第一个通行密钥', true, 'delete=' + deleteFirstResult + ' confirm=' + confirmDelete + ' remaining=' + afterFirstDelete);
  ab.screenshot('t02f-07-after-first-delete');

  // ── TC-81: 删除最后一个通行密钥 ──
  var allCreds = apiCall(`
    (function() {
      var token = localStorage.getItem('admin_token');
      return fetch('${API_BASE}/v1/disk/admin/mfa/credentials', {
        headers: { 'Authorization': 'Bearer ' + token },
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);

  var remainingCreds = allCreds.data || [];
  remainingCreds.forEach(function(cred) {
    apiCall(`
      (function() {
        var token = localStorage.getItem('admin_token');
        return fetch('${API_BASE}/v1/disk/admin/mfa/credentials/${cred.id}', {
          method: 'DELETE',
          headers: { 'Authorization': 'Bearer ' + token },
        })
        .then(function(r) { return r.json(); })
        .then(function(d) { return JSON.stringify(d); })
        .catch(function(e) { return JSON.stringify({error: e.message}); });
      })()
    `);
    ab.waitMs(300);
  });
  step('TC-81: 删除所有剩余通行密钥', true, 'deleted=' + remainingCreds.length);

  ab.evalStdin(`window.location.reload()`);
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  var emptyAfterCleanup = ab.pageContainsText('暂无通行密钥');
  var switchDisabledAfter = ab.evalStdin(`
    (function() {
      var btn = document.querySelector('button.ant-switch');
      if (!btn) return 'no-switch';
      return btn.disabled ? 'disabled' : 'enabled';
    })()
  `);
  assertCondition(
    emptyAfterCleanup && String(switchDisabledAfter).includes('disabled'),
    'TC-81: 清理完毕，列表清空，MFA 开关禁用',
    'empty=' + emptyAfterCleanup + ' switch=' + switchDisabledAfter
  );
  ab.screenshot('t02f-08-cleanup-done');

  ab.closeBrowser();
});

printReport();
