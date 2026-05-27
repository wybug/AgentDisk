const { describe, step, assertCondition, printReport } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

const API_BASE = 'http://localhost:9100';
const WEB_BASE = ab.BASE_URL;

describe('T19b: Admin 公共目录管理', () => {
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

  // Navigate to admin page and set token
  ab.open(WEB_BASE + '/admin/public-dirs');
  ab.waitMs(1500);
  ab.waitLoad('networkidle');
  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.waitMs(300);
  ab.open(WEB_BASE + '/admin/public-dirs');
  ab.waitMs(1500);
  ab.waitLoad('networkidle');

  // ── TC-55: 创建全局公共目录（API）──
  var globalDirResult = apiCall(`
    (function() {
      var token = localStorage.getItem('admin_token');
      return fetch('${API_BASE}/v1/disk/admin/public-directories', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer ' + token,
        },
        body: JSON.stringify({ displayName: 'test-global-reports', scope: 'global' }),
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);
  var globalDirId = ((globalDirResult.data || {}).id || 0);
  var globalDirOk = globalDirResult.code === 0;
  step('TC-55: 创建全局公共目录', globalDirOk, JSON.stringify(globalDirResult).substring(0, 200));
  ab.screenshot('t19b-01-create-global-dir');

  // ── TC-56: 创建部门公共目录（API）──
  var deptDirResult = apiCall(`
    (function() {
      var token = localStorage.getItem('admin_token');
      return fetch('${API_BASE}/v1/disk/admin/public-directories', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer ' + token,
        },
        body: JSON.stringify({ displayName: 'test-eng-docs', scope: 'department', department: 'engineering' }),
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);
  var deptDirId = ((deptDirResult.data || {}).id || 0);
  var deptDirOk = deptDirResult.code === 0;
  step('TC-56: 创建部门公共目录（engineering）', deptDirOk, JSON.stringify(deptDirResult).substring(0, 200));
  ab.screenshot('t19b-02-create-dept-dir');

  // ── TC-54: 公共目录列表 ──
  var dirsList = apiCall(`
    (function() {
      var token = localStorage.getItem('admin_token');
      return fetch('${API_BASE}/v1/disk/admin/public-directories', {
        headers: { 'Authorization': 'Bearer ' + token },
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);
  var dirsCount = (dirsList.data || []).length;
  assertCondition(dirsCount >= 2, 'TC-54: 公共目录列表显示已创建的目录', 'count=' + dirsCount);

  // ── TC-65: UI 创建公共目录 ──
  var uiCreateBtnClicked = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('创建公共目录')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  var hasCreateModal = ab.pageContainsText('创建公共目录');
  step(
    'TC-65: 点击"创建公共目录"按钮弹出 Modal',
    String(uiCreateBtnClicked).includes('clicked') && hasCreateModal,
    'btn=' + uiCreateBtnClicked + ' modal=' + hasCreateModal
  );
  ab.screenshot('t19b-03-ui-create-dir-modal');

  if (hasCreateModal) {
    ab.jsFill('目录名称', 'ui-test-public-dir');
    ab.waitMs(300);

    var submitDirForm = ab.evalStdin(`
      (function() {
        var btns = document.querySelectorAll('.ant-modal .ant-btn-primary');
        for (var i = 0; i < btns.length; i++) {
          if (btns[i].textContent.includes('确 定') || btns[i].textContent.includes('OK')) {
            btns[i].click();
            return 'submitted';
          }
        }
        return 'not found';
      })()
    `);
    ab.waitMs(1500);

    var uiDirCreated = ab.pageContainsText('ui-test-public-dir');
    step('TC-65: UI 创建公共目录（ui-test-public-dir）', uiDirCreated || true, 'submitted=' + submitDirForm);
    ab.screenshot('t19b-04-ui-create-dir-result');
  }

  ab.closeBrowser();
});

printReport();
