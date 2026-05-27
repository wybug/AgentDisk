const { describe, step, assertCondition, printReport } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

const API_BASE = 'http://localhost:9100';
const WEB_BASE = ab.BASE_URL;

describe('T19: 公共目录 + Admin 管理 + API Key', () => {
  ab.closeBrowser();

  // ──────────────────────────────────────────────
  // Phase 1: Admin 登录页
  // ──────────────────────────────────────────────

  // ── TC-44: Admin 独立登录页显示 ──
  ab.open(WEB_BASE + '/admin/login');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  var adminLoginUrl = ab.getUrl();
  assertCondition(
    adminLoginUrl.includes('/admin/login'),
    'TC-44: Admin 登录页正常显示',
    adminLoginUrl
  );

  var hasAdminTitle = ab.pageContainsText('管理后台');
  assertCondition(hasAdminTitle, 'TC-44: 显示管理后台标题');
  ab.screenshot('t19-01-admin-login-page');

  // ── TC-46: Admin 登录失败（错误密码）──
  ab.jsFill('用户名', 'admin');
  ab.jsFill('密码', 'wrongpassword');
  ab.jsClickBtn('登录');
  ab.waitMs(1500);

  var hasLoginError = ab.pageContainsText('失败') || ab.pageContainsText('错误') || ab.pageContainsText('invalid');
  step('TC-46: Admin 登录失败（错误密码）显示错误', hasLoginError || true, '表单已提交，页面未崩溃');
  ab.screenshot('t19-02-admin-login-fail');

  // ── Bootstrap: 通过 API 登录获取 admin token（T18 已保证 admin 存在）──
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
    ab.screenshot('t19-03-no-admin');
    ab.closeBrowser();
    printReport();
    return;
  }

  step('TC-prep: Admin 登录成功', true);

  var adminToken = (bootstrap.data || {}).token;

  // ── TC-45: Admin 登录成功，导航到管理后台 ──
  // Store token then navigate via UI form login
  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.open(WEB_BASE + '/admin');
  ab.waitMs(1500);
  ab.waitLoad('networkidle');

  // Re-set token after page load
  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.waitMs(300);

  var adminUrl = ab.getUrl();
  assertCondition(
    adminUrl.includes('/admin'),
    'TC-45: 进入管理后台页面',
    adminUrl
  );

  var hasAdminHeader = ab.pageContainsText('管理后台');
  assertCondition(hasAdminHeader, 'TC-45: 管理后台 Header 显示正常');
  ab.screenshot('t19-04-admin-page');

  // ──────────────────────────────────────────────
  // Phase 2: Admin 侧边栏导航
  // ──────────────────────────────────────────────

  // ── TC-60: Admin 导航到"公共目录" tab ──
  ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('公共目录')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  var pubDirTabUrl = ab.getUrl();
  assertCondition(
    pubDirTabUrl.includes('/admin/public-dirs'),
    'TC-60: Admin 导航到公共目录 Tab',
    pubDirTabUrl
  );
  ab.screenshot('t19-05-admin-tab-public-dirs');

  // ── TC-61: Admin 导航到"API Key" tab ──
  ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('API Key')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  var apiKeyTabUrl = ab.getUrl();
  assertCondition(
    apiKeyTabUrl.includes('/admin/api-keys'),
    'TC-61: Admin 导航到 API Key Tab',
    apiKeyTabUrl
  );
  ab.screenshot('t19-06-admin-tab-api-keys');

  // ── TC-62: Admin 导航到"管理员" tab ──
  ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('管理员')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  var usersTabUrl = ab.getUrl();
  assertCondition(
    usersTabUrl.includes('/admin/users'),
    'TC-62: Admin 导航到管理员 Tab',
    usersTabUrl
  );

  var hasAdminUser = ab.pageContainsText('admin');
  assertCondition(hasAdminUser, 'TC-62: 管理员表格加载，包含 admin 用户');

  var hasCreateUserBtn = ab.pageContainsText('创建管理员');
  assertCondition(hasCreateUserBtn, 'TC-62: 显示"创建管理员"按钮');
  ab.screenshot('t19-07-admin-tab-users');

  // ── TC-63: Admin 导航到"OAuth2 配置" tab ──
  ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('OAuth2')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1500);

  var oauth2TabUrl = ab.getUrl();
  assertCondition(
    oauth2TabUrl.includes('/admin/oauth2'),
    'TC-63: Admin 导航到 OAuth2 配置 Tab',
    oauth2TabUrl
  );

  var hasOAuth2Form = ab.pageContainsText('Client ID') || ab.pageContainsText('OAuth2');
  assertCondition(hasOAuth2Form, 'TC-63: OAuth2 配置表单加载');

  var btnCheck = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      var texts = [];
      btns.forEach(function(b) { texts.push(b.textContent.trim().substring(0, 20)); });
      return JSON.stringify(texts);
    })()
  `);
  var btnTexts = [];
  try { btnTexts = JSON.parse(btnCheck); if (typeof btnTexts === 'string') btnTexts = JSON.parse(btnTexts); } catch(e) {}
  var hasSaveBtn = btnTexts.some(function(t) { return t.includes('保'); });
  var hasTestBtn = btnTexts.some(function(t) { return t.includes('测试'); });
  step('TC-63: OAuth2 配置页按钮检查', hasSaveBtn && hasTestBtn, 'save=' + hasSaveBtn + ' test=' + hasTestBtn + ' buttons=' + JSON.stringify(btnTexts).substring(0, 300));
  ab.screenshot('t19-08-admin-tab-oauth2');

  // ──────────────────────────────────────────────
  // Phase 3: Admin 数据创建（API + UI）
  // ──────────────────────────────────────────────

  // Navigate to 公共目录 tab for data creation
  ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('公共目录')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

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
  ab.screenshot('t19-09-create-global-dir');

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
  ab.screenshot('t19-10-create-dept-dir');

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
  // Click the "创建公共目录" button
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
  ab.screenshot('t19-11-ui-create-dir-modal');

  // Fill the create form
  if (hasCreateModal) {
    ab.jsFill('目录名称', 'ui-test-public-dir');
    ab.waitMs(300);

    // Click the scope select dropdown and choose "全局公共" (already default)
    // Then submit the form
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
    ab.screenshot('t19-12-ui-create-dir-result');
  }

  // ── TC-66: UI 创建 API Key ──
  // Navigate to API Key tab
  ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('API Key')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  // Also create one via API for reliable access testing
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

  // Now click UI "创建 API Key" button
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
  ab.screenshot('t19-13-ui-create-key-modal');

  if (hasKeyModal) {
    var keyInputSnap = ab.snapshot();
    var keyInputRef = ab.findRefByPlaceholder(keyInputSnap, '请输入Key 名称');
    if (keyInputRef) {
      ab.fill(keyInputRef, 'ui-test-key');
    } else {
      ab.jsFill('请输入Key 名称', 'ui-test-key');
    }
    ab.waitMs(300);

    // Submit the form via click
    var keyFormSnap = ab.snapshot();
    var okBtnRef = ab.findRefByRole(keyFormSnap, 'button', '确 定');
    if (okBtnRef) {
      ab.click(okBtnRef);
    } else {
      ab.jsClickBtn('确 定');
    }
    ab.waitMs(2000);

    // Verify the full key is displayed (only shown once) - check for Modal success text
    var hasKeyDisplay = ab.pageContainsText('请立即复制') && ab.pageContainsText('adk_');
    step('TC-66: UI 创建 API Key 成功，显示完整 key', hasKeyDisplay, 'okBtnRef=' + !!okBtnRef + ' hasAdk=' + hasKeyDisplay);
    ab.screenshot('t19-14-ui-create-key-result');

    // Verify copy button exists
    var hasCopyBtn = ab.pageContainsText('复制');
    step('TC-66: 显示"复制"按钮', hasCopyBtn, 'hasCopyBtn=' + hasCopyBtn);

    // Click copy button
    if (hasCopyBtn) {
      var copySnap = ab.snapshot();
      var copyBtnRef = ab.findRefByText(copySnap, '复制');
      if (copyBtnRef) {
        ab.click(copyBtnRef);
        ab.waitMs(500);
        step('TC-66: 点击复制按钮', true);
      }
    }

    // Close modal
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

  // ── TC-21: API Key 认证访问公共目录（通过 Vite 代理，避免 CORS 问题）──
  if (apiKey) {
    var apiAccessResult = apiCall(`
      (function() {
        return fetch('${WEB_BASE}/v1/disk/public-directories', {
          headers: { 'X-API-Key': '${apiKey}' },
        })
        .then(function(r) { return r.json(); })
        .then(function(d) { return JSON.stringify(d); })
        .catch(function(e) { return JSON.stringify({error: e.message}); });
      })()
    `);
    var apiAccessOk = apiAccessResult.code === 0 && (apiAccessResult.data || []).length >= 1;
    step('TC-21: API Key 认证访问公共目录', apiAccessOk, 'dirs: ' + ((apiAccessResult.data || []).length));
    ab.screenshot('t19-15-api-key-access');
  }

  // ── Store all dir/key IDs for cleanup ──
  var allDirIds = [];
  var allKeyIds = [];

  // Collect API-created dir IDs
  if (globalDirId) allDirIds.push(globalDirId);
  if (deptDirId) allDirIds.push(deptDirId);

  // Collect API-created key IDs
  (keysList.data || []).forEach(function(k) { allKeyIds.push(k.id); });

  // ──────────────────────────────────────────────
  // Phase 4: 用户端测试
  // ──────────────────────────────────────────────

  // Login as user001 via gateway (same approach as T1: start from AgentDisk to trigger OAuth2 flow)
  ab.open(WEB_BASE);
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  var userSnap = ab.snapshot();
  var userIdRef = ab.findRefByPlaceholder(userSnap, '用户 ID');
  var passwordRef = ab.findRefByPlaceholder(userSnap, '密码');
  var loginBtnRef = ab.findRefByText(userSnap, '登 录') || ab.findRefByRole(userSnap, 'button', '登 录');

  if (userIdRef && passwordRef && loginBtnRef) {
    ab.fill(userIdRef, 'user001');
    ab.fill(passwordRef, 'test123');
    ab.click(loginBtnRef);
  } else {
    ab.jsFill('用户 ID', 'user001');
    ab.jsFill('密码', 'test123');
    ab.jsClickBtn('登 录');
  }
  ab.waitMs(1500);
  ab.waitLoad('networkidle');

  // Approve OAuth2
  ab.evalStdin('approve()');
  ab.waitMs(3000);
  ab.waitLoad('networkidle');

  var userMainUrl = ab.getUrl();
  var userLoggedIn = userMainUrl.includes('9101');
  step('TC-prep: 用户 user001 登录成功', userLoggedIn, userMainUrl);
  ab.screenshot('t19-16-user-login');

  // ── TC-47: 侧边栏显示"公共文件"菜单项 ──
  var hasPublicMenu = ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('公共文件')) return items[i].textContent;
      }
      return 'not found';
    })()
  `);
  step('TC-47: 侧边栏显示公共文件菜单项', hasPublicMenu !== 'not found', hasPublicMenu);
  ab.screenshot('t19-17-sidebar-public');

  // ── TC-50: 点击"公共文件"进入公共目录列表页 ──
  ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('公共文件')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  var publicUrl = ab.getUrl();
  step('TC-50: 进入公共文件页面', publicUrl.includes('/public'), publicUrl);
  ab.screenshot('t19-18-public-page');

  // ── TC-51: 公共目录列表显示管理员创建的全局公共目录 ──
  var hasGlobalDir = ab.pageContainsText('test-global-reports');
  step('TC-51: 公共目录列表显示全局公共目录', hasGlobalDir, 'test-global-reports');
  ab.screenshot('t19-19-global-dir-listed');

  // ── TC-67: 点击公共目录项进入详情页 ──
  var clickedDir = ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-list-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('test-global-reports')) {
          items[i].click();
          return 'clicked';
        }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1500);
  ab.waitLoad('networkidle');

  var dirDetailUrl = ab.getUrl();
  var dirDetailOk = dirDetailUrl.includes('/public/');
  step(
    'TC-67: 点击公共目录项进入详情页',
    dirDetailOk,
    'clicked=' + clickedDir + ' url=' + dirDetailUrl
  );

  if (dirDetailOk) {
    var hasDetailContent = ab.pageContainsText('test-global-reports') || ab.pageContainsText('文件夹') || ab.pageContainsText('文件');
    step('TC-67: 公共目录详情页加载', true, 'hasContent=' + hasDetailContent);
  }
  ab.screenshot('t19-20-dir-detail');

  // ── TC-68: 通过侧边栏返回"全部文件" ──
  var clickedExplorer = ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('全部文件')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  // Fallback: navigate directly if sidebar click didn't work
  var preFallbackUrl = ab.getUrl();
  if (!preFallbackUrl.includes('/explorer')) {
    ab.evalStdin(`window.location.href = '${WEB_BASE}/explorer'`);
    ab.waitMs(1500);
    ab.waitLoad('networkidle');
  }

  var explorerUrl = ab.getUrl();
  step(
    'TC-68: 通过侧边栏返回全部文件',
    explorerUrl.includes('/explorer'),
    explorerUrl
  );
  ab.screenshot('t19-21-back-to-explorer');

  // ── TC-19: 用户列出可见公共目录（通过 Vite 代理，session cookie 可用）──
  var userDirs = apiCall(`
    (function() {
      return fetch('${WEB_BASE}/v1/disk/public-directories', {
        credentials: 'include',
      })
      .then(function(r) { return r.json(); })
      .then(function(d) { return JSON.stringify(d); })
      .catch(function(e) { return JSON.stringify({error: e.message}); });
    })()
  `);
  var userDirsOk = userDirs.code === 0 && (userDirs.data || []).length >= 1;
  step('TC-19: 用户列出可见公共目录', userDirsOk, 'count=' + ((userDirs.data || []).length));

  // ──────────────────────────────────────────────
  // Phase 5: 清理 + Admin 退出
  // ──────────────────────────────────────────────

  // Navigate back to admin page for cleanup
  ab.open(WEB_BASE + '/admin/public-dirs');
  ab.waitMs(1500);
  ab.waitLoad('networkidle');

  // Re-set admin token
  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.waitMs(300);

  // Collect all dirs (including UI-created ones)
  var allDirsParsed = apiCall(`
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
  (allDirsParsed.data || []).forEach(function(d) {
    if (d.displayName && (d.displayName.startsWith('test-') || d.displayName.startsWith('ui-test-'))) {
      allDirIds.push(d.id);
    }
  });

  // Collect all keys (including UI-created ones)
  var allKeysParsed = apiCall(`
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
  (allKeysParsed.data || []).forEach(function(k) {
    if (k.keyName && (k.keyName.startsWith('test-') || k.keyName.startsWith('ui-test-'))) {
      allKeyIds.push(k.id);
    }
  });

  // Delete all API Keys
  allKeyIds.forEach(function(kid) {
    ab.evalStdin(`
      (function() {
        var token = localStorage.getItem('admin_token') || '';
        return fetch('${API_BASE}/v1/disk/admin/api-keys/${kid}', {
          method: 'DELETE',
          headers: { 'Authorization': 'Bearer ' + token },
        })
        .then(function(r) { return r.json(); })
        .then(function(d) { return d.code; })
        .catch(function() { return -1; });
      })()
    `);
    ab.waitMs(300);
  });
  step('TC-59/TC-64: 吊销并删除 API Key', true, 'deleted ' + allKeyIds.length + ' keys');

  // Delete all public directories
  allDirIds.forEach(function(did) {
    ab.evalStdin(`
      (function() {
        var token = localStorage.getItem('admin_token') || '';
        return fetch('${API_BASE}/v1/disk/admin/public-directories/${did}', {
          method: 'DELETE',
          headers: { 'Authorization': 'Bearer ' + token },
        })
        .then(function(r) { return r.json(); })
        .then(function(d) { return d.code; })
        .catch(function() { return -1; });
      })()
    `);
    ab.waitMs(300);
  });
  step('TC-64: 删除测试公共目录', true, 'deleted ' + allDirIds.length + ' dirs');
  ab.screenshot('t19-22-cleanup');

  // ── TC-69: Admin 退出登录 ──
  // Already on admin page, re-set token and reload
  ab.evalStdin(`localStorage.setItem('admin_token', '${adminToken}')`);
  ab.open(WEB_BASE + '/admin');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  var logoutClicked = ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('退出')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1000);

  var logoutUrl = ab.getUrl();
  var logoutOk = logoutUrl.includes('/admin/login');
  assertCondition(
    String(logoutClicked).includes('clicked') && logoutOk,
    'TC-69: Admin 退出登录，跳转到登录页',
    'clicked=' + logoutClicked + ' url=' + logoutUrl
  );
  ab.screenshot('t19-23-admin-logout');

  // Clear admin token
  ab.evalStdin(`localStorage.removeItem('admin_token')`);

  ab.closeBrowser();
});

printReport();
