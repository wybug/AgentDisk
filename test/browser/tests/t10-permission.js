const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

function findFileId() {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files?folderId=0', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          return items.length > 0 ? items[0].id : 'none';
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function grantPermAPI(agentId, resourceId, resType, permission) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/permissions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agentId: '${agentId}', resourceId: ${resourceId}, resType: '${resType}', permission: '${permission}' }),
        credentials: 'include'
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: granted';
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function listPermsAPI() {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/permissions', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          var items = d.data || [];
          return 'OK: ' + items.length + ' perms: ' + items.map(function(p) {
            return p.agentId + ':' + p.permission;
          }).join(', ');
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function checkPermAPI(agentId, resourceId, resType, permission) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/permissions/check?agentId=${agentId}&resourceId=${resourceId}&resType=${resType}&permission=${permission}', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: allowed=' + d.data.allowed;
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function revokePermAPI(agentId, resourceId, resType) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/permissions', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agentId: '${agentId}', resourceId: ${resourceId}, resType: '${resType}' }),
        credentials: 'include'
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: revoked';
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function navigateTo(page) {
  return ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('${page}')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
}

describe('T10: 权限管理', () => {
  ab.closeAll();
  ab.login('user001', 'test123');
  ab.waitMs(2000);

  // T10.1 - 导航到权限管理页面
  navigateTo('权限管理');
  ab.waitMs(2000);
  const urlPerm = ab.getUrl();
  assertCondition(urlPerm.includes('/permissions'), 'T10.1: 导航到权限管理页面', urlPerm);
  ab.screenshot('t10-01-permissions-page');

  // T10.2 - 获取文件 ID 用于授权
  const fileId = findFileId();
  ab.waitMs(1000);
  assertCondition(fileId !== 'none' && !fileId.includes('ERR'), 'T10.2: 获取文件 ID', 'fileId=' + fileId);

  // T10.3 - 授予权限
  const grantResult = grantPermAPI('agent-test-001', fileId, 'file', 'read');
  ab.waitMs(1000);
  assertCondition(grantResult.includes('OK'), 'T10.3: 授予权限成功', grantResult);
  ab.screenshot('t10-03-granted');

  // T10.4 - 验证权限记录
  const listResult = listPermsAPI();
  ab.waitMs(1000);
  const hasRecord = listResult.includes('agent-test-001');
  assertCondition(hasRecord, 'T10.4: 权限列表显示记录', listResult);
  ab.screenshot('t10-04-permission-list');

  // T10.5 - 检查权限
  const checkResult = checkPermAPI('agent-test-001', fileId, 'file', 'read');
  ab.waitMs(1000);
  assertCondition(checkResult.includes('allowed=true'), 'T10.5: 权限检查通过', checkResult);

  // T10.6 - 撤销权限
  const revokeResult = revokePermAPI('agent-test-001', fileId, 'file');
  ab.waitMs(1000);
  assertCondition(revokeResult.includes('OK'), 'T10.6: 撤销权限成功', revokeResult);
  ab.screenshot('t10-06-revoked');

  // T10.7 - 验证撤销后权限不存在（allowed=false 或 查询出错均表示无权限）
  const checkAfter = checkPermAPI('agent-test-001', fileId, 'file', 'read');
  ab.waitMs(1000);
  const notAllowed = checkAfter.includes('allowed=false') || checkAfter.includes('ERROR');
  assertCondition(notAllowed, 'T10.7: 撤销后权限已失效', checkAfter);

  ab.closeBrowser();
});
