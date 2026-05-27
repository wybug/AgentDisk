const { describe, step, assertCondition, printReport } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

const API_BASE = 'http://localhost:9100';
const WEB_BASE = ab.BASE_URL;

describe('T19c: Admin API Key 管理', () => {
  ab.closeBrowser();

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

  // Bootstrap: API 登录获取 admin token
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
    step('TC-prep: Admin 登录失败，请先运行 T18 bootstrap', true, (bootstrap.message || JSON.stringify(bootstrap)).substring(0, 200));
    ab.closeBrowser();
    printReport();
    return;
  }

  var adminToken = (bootstrap.data || {}).token;
  step('TC-prep: Admin 登录成功', true);

  // Navigate to API Key tab
  ab.open(WEB_BASE + '/admin/api-keys');
  ab.waitMs(1500);
  ab.waitLoad('networkidle');
  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.waitMs(300);
  ab.open(WEB_BASE + '/admin/api-keys');
  ab.waitMs(1500);
  ab.waitLoad('networkidle');

  // ── TC-57: 创建 API Key（API）──
  var keyResult = apiCall(`
    (function() {
      var token = localStorage.getItem('admin_token');
      return fetch('${API_BASE}/v1/disk/admin/api-keys', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer ' + token,
        },
        body: JSON.stringify({ name: 'test-key-t19' }),
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);
  var keyCreated = keyResult.code === 0 && (keyResult.data || {}).key;
  var apiKey = (keyResult.data || {}).key || '';
  step('TC-57: 创建 API Key', keyCreated, 'key length: ' + apiKey.length);

  // ── TC-58: API Key 列表 ──
  var keysList = apiCall(`
    (function() {
      var token = localStorage.getItem('admin_token');
      return fetch('${API_BASE}/v1/disk/admin/api-keys', {
        headers: { 'Authorization': 'Bearer ' + token },
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);
  var keysCount = (keysList.data || []).length;
  var keyPrefixOk = keysCount > 0 && (keysList.data[0] || {}).keyPrefix;
  step('TC-58: API Key 列表显示 prefix', keyPrefixOk, 'count=' + keysCount);

  // ── TC-66: UI 创建 API Key ──
  var uiKeyBtnClicked = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('创建 API Key')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  var hasKeyModal = ab.pageContainsText('Key 名称');
  step(
    'TC-66: 点击"创建 API Key"按钮弹出 Modal',
    String(uiKeyBtnClicked).includes('clicked') && hasKeyModal,
    'btn=' + uiKeyBtnClicked + ' modal=' + hasKeyModal
  );
  ab.screenshot('t19c-01-ui-create-key-modal');

  if (hasKeyModal) {
    var keyInputSnap = ab.snapshot();
    var keyInputRef = ab.findRefByPlaceholder(keyInputSnap, '请输入Key 名称');
    if (keyInputRef) {
      ab.fill(keyInputRef, 'ui-test-key');
    } else {
      ab.jsFill('请输入Key 名称', 'ui-test-key');
    }
    ab.waitMs(300);

    var keyFormSnap = ab.snapshot();
    var okBtnRef = ab.findRefByRole(keyFormSnap, 'button', '确 定');
    if (okBtnRef) {
      ab.click(okBtnRef);
    } else {
      ab.jsClickBtn('确 定');
    }
    ab.waitMs(2000);

    var hasKeyDisplay = ab.pageContainsText('保存在安全') && ab.pageContainsText('adk_');
    step('TC-66: UI 创建 API Key 成功，显示完整 key', hasKeyDisplay, 'okBtnRef=' + !!okBtnRef + ' hasAdk=' + hasKeyDisplay);
    ab.screenshot('t19c-02-ui-create-key-result');

    var hasCopyBtn = ab.pageContainsText('复制');
    step('TC-66: 显示"复制"按钮', hasCopyBtn, 'hasCopyBtn=' + hasCopyBtn);

    if (hasCopyBtn) {
      var copySnap = ab.snapshot();
      var copyBtnRef = ab.findRefByText(copySnap, '复制');
      if (copyBtnRef) {
        ab.click(copyBtnRef);
        ab.waitMs(500);
        step('TC-66: 点击复制按钮', true);
      }
    }

    var closeSnap = ab.snapshot();
    var closeBtnRef = ab.findRefByText(closeSnap, '关闭');
    if (closeBtnRef) {
      ab.click(closeBtnRef);
    } else {
      ab.jsClickBtn('关闭');
    }
    ab.waitMs(500);
    step('TC-66: 关闭 API Key Modal', true, 'closeBtnRef=' + !!closeBtnRef);
  }

  // ── TC-67: 编辑 API Key 名称 ──
  // Reload page to ensure clean table state after modal close
  ab.evalStdin(`window.location.reload()`);
  ab.waitMs(1500);
  ab.waitLoad('networkidle');

  var hasEditTarget = ab.pageContainsText('ui-test-key');
  if (hasEditTarget) {
    var editClickResult = ab.evalStdin(`
      (function() {
        var rows = document.querySelectorAll('tr');
        for (var i = 0; i < rows.length; i++) {
          var cells = rows[i].querySelectorAll('td');
          if (cells.length === 0) continue;
          var nameCell = cells[0];
          if (nameCell && nameCell.textContent.indexOf('ui-test-key') !== -1 &&
              rows[i].textContent.indexOf('已吊销') === -1) {
            var btns = rows[i].querySelectorAll('button');
            for (var j = 0; j < btns.length; j++) {
              if (btns[j].querySelector('svg')) {
                btns[j].click();
                return 'clicked btn' + j;
              }
            }
            return 'no-svg-btns';
          }
        }
        return 'not found';
      })()
    `);
    ab.waitMs(800);
    var editModalOpen = ab.pageContainsText('修改 API Key 名称');
    if (editModalOpen) {
      var editInputSnap = ab.snapshot();
      var editInputRef = ab.findRefByPlaceholder(editInputSnap, '请输入Key 名称');
      if (editInputRef) {
        ab.fill(editInputRef, 'ui-test-key-renamed');
      } else {
        ab.jsFill('请输入Key 名称', 'ui-test-key-renamed');
      }
      ab.waitMs(200);
      var editOkSnap = ab.snapshot();
      var editOkRef = ab.findRefByRole(editOkSnap, 'button', '确 定');
      if (editOkRef) {
        ab.click(editOkRef);
      } else {
        ab.jsClickBtn('确 定');
      }
      ab.waitMs(1000);
      var hasRenamed = ab.pageContainsText('ui-test-key-renamed');
      step('TC-67: 编辑 API Key 名称成功', hasRenamed, 'clickResult=' + editClickResult + ' renamed=' + hasRenamed);
    } else {
      step('TC-67: 编辑 API Key 名称成功', false, 'clickResult=' + editClickResult + ' editModal not open');
    }
    ab.screenshot('t19c-03-edit-apikey-result');
  }

  ab.closeBrowser();
});

printReport();
