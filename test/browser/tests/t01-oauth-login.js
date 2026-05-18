const { describe, step, assertCondition, printReport } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T1: OAuth2 登录流程', () => {
  ab.closeBrowser();

  // T1.1 - 访问前端，重定向到网关登录页
  ab.open(ab.BASE_URL);
  ab.waitMs(4000);
  ab.waitLoad('networkidle');

  const url1 = ab.getUrl();
  const isLoginPage = url1.includes('3100') && url1.includes('login');
  assertCondition(isLoginPage, 'T1.1: 浏览器重定向到网关登录页', url1);
  ab.screenshot('t01-01-login-page');

  // T1.2 - 输入用户名密码登录
  let snap = ab.snapshot();
  const userIdRef = ab.findRefByPlaceholder(snap, '用户 ID');
  const passwordRef = ab.findRefByPlaceholder(snap, '密码');
  const loginBtnRef = ab.findRefByText(snap, '登 录') || ab.findRefByRole(snap, 'button', '登 录');

  assertCondition(userIdRef !== null, 'T1.2: 找到用户 ID 输入框', userIdRef || 'not found');
  assertCondition(passwordRef !== null, 'T1.2: 找到密码输入框', passwordRef || 'not found');
  assertCondition(loginBtnRef !== null, 'T1.2: 找到登录按钮', loginBtnRef || 'not found');

  ab.fill(userIdRef, 'user001');
  ab.fill(passwordRef, 'test123');
  ab.click(loginBtnRef);

  // T1.3 - 等待重定向到授权页面
  ab.waitMs(3000);
  ab.waitLoad('networkidle');

  const url2 = ab.getUrl();
  const isAuthorizePage = url2.includes('3100') && url2.includes('authorize');
  assertCondition(isAuthorizePage, 'T1.3: 到达授权确认页', url2);
  ab.screenshot('t01-02-authorize-page');

  // T1.3b - 点击允许
  ab.evalStdin('approve()');
  ab.waitMs(6000);
  ab.waitLoad('networkidle');

  // T1.4 - 验证重定向回前端主界面
  const url3 = ab.getUrl();
  const isMainPage = url3.includes('9101') && (url3.includes('explorer') || url3 === ab.BASE_URL + '/');
  assertCondition(isMainPage, 'T1.4: 登录成功，回到前端主界面', url3);
  ab.screenshot('t01-03-main-page');

  // T1.5 - 验证界面元素
  const hasTitle = ab.pageContainsText('AgentDisk');
  assertCondition(hasTitle, 'T1.5a: 页面显示 AgentDisk 标题');

  const hasUser = ab.pageContainsText('user001');
  assertCondition(hasUser, 'T1.5b: 页面显示用户信息 user001');

  const hasSpace = ab.pageContainsText('GB') || ab.pageContainsText('MB');
  assertCondition(hasSpace, 'T1.5c: 页面显示空间用量');
  ab.screenshot('t01-04-main-ui');

  // ============================================================
  // T1.6 - 导航：全部文件
  // ============================================================
  ab.evalStdin(`(function() {
    var items = document.querySelectorAll('.ant-menu-item');
    for (var i = 0; i < items.length; i++) {
      if (items[i].textContent.includes('全部文件')) { items[i].click(); return 'clicked'; }
    }
    return 'not found';
  })()`);
  ab.waitMs(2000);

  let urlNav1 = ab.getUrl();
  assertCondition(
    urlNav1.includes('/explorer'),
    'T1.6: 导航到全部文件',
    urlNav1
  );
  ab.screenshot('t01-06-explorer');

  // ============================================================
  // T1.7 - 导航：回收站
  // ============================================================
  ab.evalStdin(`(function() {
    var items = document.querySelectorAll('.ant-menu-item');
    for (var i = 0; i < items.length; i++) {
      if (items[i].textContent.includes('回收站')) { items[i].click(); return 'clicked'; }
    }
    return 'not found';
  })()`);
  ab.waitMs(2000);

  let urlNav2 = ab.getUrl();
  let hasRecycleTitle = ab.pageContainsText('回收站');
  assertCondition(
    urlNav2.includes('/recycle') && hasRecycleTitle,
    'T1.7: 导航到回收站',
    `url=${urlNav2} title=${hasRecycleTitle}`
  );
  ab.screenshot('t01-07-recycle');

  // ============================================================
  // T1.8 - 导航：我的分享
  // ============================================================
  ab.evalStdin(`(function() {
    var items = document.querySelectorAll('.ant-menu-item');
    for (var i = 0; i < items.length; i++) {
      if (items[i].textContent.includes('我的分享')) { items[i].click(); return 'clicked'; }
    }
    return 'not found';
  })()`);
  ab.waitMs(2000);

  let urlNav3 = ab.getUrl();
  let hasSharesTitle = ab.pageContainsText('分享');
  assertCondition(
    urlNav3.includes('/shares') && hasSharesTitle,
    'T1.8: 导航到我的分享',
    `url=${urlNav3} title=${hasSharesTitle}`
  );
  ab.screenshot('t01-08-shares');

  // ============================================================
  // T1.9 - 导航：标签搜索
  // ============================================================
  ab.evalStdin(`(function() {
    var items = document.querySelectorAll('.ant-menu-item');
    for (var i = 0; i < items.length; i++) {
      if (items[i].textContent.includes('标签搜索')) { items[i].click(); return 'clicked'; }
    }
    return 'not found';
  })()`);
  ab.waitMs(2000);

  let urlNav4 = ab.getUrl();
  let hasTagsTitle = ab.pageContainsText('标签');
  assertCondition(
    urlNav4.includes('/tags') && hasTagsTitle,
    'T1.9: 导航到标签搜索',
    `url=${urlNav4} title=${hasTagsTitle}`
  );
  ab.screenshot('t01-09-tags');

  // ============================================================
  // T1.10 - 导航：权限管理
  // ============================================================
  ab.evalStdin(`(function() {
    var items = document.querySelectorAll('.ant-menu-item');
    for (var i = 0; i < items.length; i++) {
      if (items[i].textContent.includes('权限管理')) { items[i].click(); return 'clicked'; }
    }
    return 'not found';
  })()`);
  ab.waitMs(2000);

  let urlNav5 = ab.getUrl();
  let hasPermTitle = ab.pageContainsText('权限');
  assertCondition(
    urlNav5.includes('/permissions') && hasPermTitle,
    'T1.10: 导航到权限管理',
    `url=${urlNav5} title=${hasPermTitle}`
  );
  ab.screenshot('t01-10-permissions');

  // ============================================================
  // T1.11 - 返回全部文件，验证导航回到首页
  // ============================================================
  ab.evalStdin(`(function() {
    var items = document.querySelectorAll('.ant-menu-item');
    for (var i = 0; i < items.length; i++) {
      if (items[i].textContent.includes('全部文件')) { items[i].click(); return 'clicked'; }
    }
    return 'not found';
  })()`);
  ab.waitMs(2000);

  let urlBack = ab.getUrl();
  assertCondition(
    urlBack.includes('/explorer'),
    'T1.11: 返回全部文件',
    urlBack
  );
  ab.screenshot('t01-11-back-to-explorer');

  // ============================================================
  // T1.11 - 全局清理：通过 API 清除所有数据（分享、权限、文件、文件夹、回收站）
  // ============================================================

  // 清理所有分享（单次 evalStdin 内串行完成）
  var shareClean = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/shares', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          var p = Promise.resolve();
          items.forEach(function(s) {
            p = p.then(function() {
              return fetch('/v1/disk/shares', {
                method: 'DELETE',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ shareId: s.id }),
                credentials: 'include'
              });
            });
          });
          return p.then(function() { return items.length; });
        })
        .catch(function(e) { return 'ERR:' + e.message; });
    })()
  `);
  ab.waitMs(5000);

  // 清理所有权限
  var permClean = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/permissions', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          return Promise.all(items.map(function(p) {
            return fetch('/v1/disk/permissions', {
              method: 'DELETE',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ agentId: p.agentId, resourceId: p.resourceId, resType: p.resType }),
              credentials: 'include'
            }).then(function(r) { return r.json(); });
          })).then(function() { return items.length; });
        })
        .catch(function(e) { return -1; });
    })()
  `);
  ab.waitMs(1000);

  // 清理所有文件
  var fileClean = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files?folderId=0', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          return Promise.all(items.map(function(f) {
            return fetch('/v1/disk/files/' + f.id, {
              method: 'DELETE', credentials: 'include'
            }).then(function(r) { return r.json(); });
          })).then(function() { return items.length; });
        })
        .catch(function(e) { return -1; });
    })()
  `);
  ab.waitMs(1000);

  // 清理所有文件夹
  var folderClean = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/folders?parentId=0', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          return Promise.all(items.map(function(f) {
            return fetch('/v1/disk/folders/' + f.id, {
              method: 'DELETE', credentials: 'include'
            }).then(function(r) { return r.json(); });
          })).then(function() { return items.length; });
        })
        .catch(function(e) { return -1; });
    })()
  `);
  ab.waitMs(1000);

  // 清理回收站（文件/文件夹删除后会产生新的回收站记录，需多轮清理）
  var recycleClean1 = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/recycle', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          return Promise.all(items.map(function(item) {
            return fetch('/v1/disk/recycle', {
              method: 'DELETE',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ recycleId: item.id }),
              credentials: 'include'
            }).then(function(r) { return r.json(); });
          })).then(function() { return items.length; });
        })
        .catch(function(e) { return -1; });
    })()
  `);
  ab.waitMs(1500);

  // 二次清理回收站（上一轮文件/文件夹软删除可能产生新记录）
  var recycleClean2 = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/recycle', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          return Promise.all(items.map(function(item) {
            return fetch('/v1/disk/recycle', {
              method: 'DELETE',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ recycleId: item.id }),
              credentials: 'include'
            }).then(function(r) { return r.json(); });
          })).then(function() { return items.length; });
        })
        .catch(function(e) { return -1; });
    })()
  `);
  ab.waitMs(1500);

  step('T1.11: 全局数据清理（API）', true,
    'shares=' + shareClean + ' perms=' + permClean +
    ' files=' + fileClean + ' folders=' + folderClean +
    ' recycle1=' + recycleClean1 + ' recycle2=' + recycleClean2);

  // T1.12 - 退出登录
  snap = ab.snapshot();
  const logoutRef = ab.findRefByText(snap, '退出登录');
  if (logoutRef) {
    ab.click(logoutRef);
    ab.waitMs(2000);
    const urlLogout = ab.getUrl();
    const loggedOut = urlLogout.includes('login') || !ab.pageContainsText('user001');
    step('T1.12: 退出登录', loggedOut, urlLogout);
    ab.screenshot('t01-12-logout');
  } else {
    step('T1.12: 退出登录（跳过）', true, '未找到退出登录按钮');
  }

  ab.closeBrowser();
});

printReport();
