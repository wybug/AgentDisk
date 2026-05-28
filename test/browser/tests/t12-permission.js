const { describe, assertCondition } = require('../lib/test-runner');
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

function findFileByName(name) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files?folderId=0', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          var found = items.find(function(f) { return f.fileName === '${name}'; });
          return found ? found.id : 'none';
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

function grantPathPermAPI(agentConfig, resourcePath, permission) {
  var body = Object.assign({}, agentConfig, { resourcePath: resourcePath, permission: permission });
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/permissions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(${JSON.stringify(body)}),
        credentials: 'include'
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: granted path';
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
            return (p.agentId || '') + ':' + (p.resourcePath || p.resourceId) + ':' + p.permission;
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

function revokePathPermAPI(agentConfig, resourcePath) {
  var body = Object.assign({}, agentConfig, { resourcePath: resourcePath });
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/permissions', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(${JSON.stringify(body)}),
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

describe('T12: 权限管理', () => {
  ab.closeAll();
  ab.login('user001', 'test123');
  ab.waitMs(2000);

  // Upload a test file for permission tests (in case T12 runs standalone)
  ab.evalStdin(`
    (function() {
      var blob = new Blob(['permission test file content'], { type: 'text/plain' });
      var fd = new FormData();
      fd.append('file', blob, 'perm-test.txt');
      return fetch('/v1/disk/files/upload?folderId=0', {
        method: 'POST', body: fd, credentials: 'include'
      }).then(function(r) { return r.json(); })
        .then(function(d) { return d.data ? d.data.id : 'ERR'; })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
  ab.waitMs(1000);

  // T12.1 - 导航到权限管理页面
  navigateTo('权限管理');
  ab.waitMs(2000);
  const urlPerm = ab.getUrl();
  assertCondition(urlPerm.includes('/permissions'), 'T12.1: 导航到权限管理页面', urlPerm);
  ab.screenshot('t10-01-permissions-page');

  // T12.2 - 获取文件 ID 用于授权
  const fileId = findFileByName('perm-test.txt');
  ab.waitMs(1000);
  assertCondition(fileId !== 'none' && !fileId.includes('ERR'), 'T12.2: 获取文件 ID', 'fileId=' + fileId);

  // T12.3 - 授予权限（资源 ID 模式）
  const grantResult = grantPermAPI('agent-test-001', fileId, 'file', 'read');
  ab.waitMs(1000);
  assertCondition(grantResult.includes('OK'), 'T12.3: 资源ID授权成功', grantResult);
  ab.screenshot('t10-03-granted');

  // T12.4 - 验证权限记录
  const listResult = listPermsAPI();
  ab.waitMs(1000);
  const hasRecord = listResult.includes('agent-test-001');
  assertCondition(hasRecord, 'T12.4: 权限列表显示记录', listResult);
  ab.screenshot('t10-04-permission-list');

  // T12.5 - 检查权限
  const checkResult = checkPermAPI('agent-test-001', fileId, 'file', 'read');
  ab.waitMs(1000);
  assertCondition(checkResult.includes('allowed=true'), 'T12.5: 权限检查通过', checkResult);

  // T12.6 - 撤销权限
  const revokeResult = revokePermAPI('agent-test-001', fileId, 'file');
  ab.waitMs(1000);
  assertCondition(revokeResult.includes('OK'), 'T12.6: 撤销权限成功', revokeResult);
  ab.screenshot('t10-06-revoked');

  // T12.7 - 验证撤销后权限不存在
  const checkAfter = checkPermAPI('agent-test-001', fileId, 'file', 'read');
  ab.waitMs(1000);
  const notAllowed = checkAfter.includes('allowed=false') || checkAfter.includes('ERROR');
  assertCondition(notAllowed, 'T12.7: 撤销后权限已失效', checkAfter);

  // T12.8 - 路径授权：授予 agent-test-002 对 /** 的 read 权限
  const pathGrantResult = grantPathPermAPI({ agentId: 'agent-test-002' }, '/**', 'read');
  ab.waitMs(1000);
  assertCondition(pathGrantResult.includes('OK'), 'T12.8: 路径授权成功', pathGrantResult);
  ab.screenshot('t10-08-path-granted');

  // T12.9 - 路径权限验证：路径授权生效
  const pathCheckResult = checkPermAPI('agent-test-002', fileId, 'file', 'read');
  ab.waitMs(1000);
  assertCondition(pathCheckResult.includes('allowed=true'), 'T12.9: 路径权限检查通过', pathCheckResult);

  // T12.10 - AgentGroup 路径授权
  const groupGrantResult = grantPathPermAPI({ agentGroupId: 'group-test-001' }, '/Documents/**', 'write');
  ab.waitMs(1000);
  assertCondition(groupGrantResult.includes('OK'), 'T12.10: 组路径授权成功', groupGrantResult);
  ab.screenshot('t10-10-group-path-granted');

  // T12.11 - 验证权限列表包含路径和组授权
  const listResult2 = listPermsAPI();
  ab.waitMs(1000);
  assertCondition(listResult2.includes('/**'), 'T12.11: 权限列表包含路径 /**', listResult2);
  assertCondition(listResult2.includes('/Documents/**'), 'T12.11: 权限列表包含路径 /Documents/**', listResult2);
  ab.screenshot('t10-11-list-with-path');

  // T12.12 - 通配符匹配：/**/*.txt 只匹配 txt 文件
  const txtGrantResult = grantPathPermAPI({ agentId: 'agent-test-003' }, '/**/*.txt', 'read');
  ab.waitMs(1000);
  assertCondition(txtGrantResult.includes('OK'), 'T12.12: txt 路径授权成功', txtGrantResult);

  // T12.13 - 路径权限撤销
  const pathRevokeResult = revokePathPermAPI({ agentId: 'agent-test-002' }, '/**');
  ab.waitMs(1000);
  assertCondition(pathRevokeResult.includes('OK'), 'T12.13: 路径权限撤销成功', pathRevokeResult);
  ab.screenshot('t10-13-path-revoked');

  // T12.14 - 权限页面路径授权 UI
  navigateTo('权限管理');
  ab.waitMs(2000);
  // Fill in the agent config JSON
  ab.jsFill('Agent 配置', '{"agentId":"agent-ui-test"}');
  ab.waitMs(500);
  // Fill in the resource path
  ab.jsFill('资源路径', '/TestFolder/**');
  ab.waitMs(500);
  ab.screenshot('t10-14-form-filled');
  // Click grant button
  ab.jsClickBtn('授予');
  ab.waitMs(2000);
  ab.screenshot('t10-14-after-grant');

  // T12.15 - 文件浏览器快捷授权
  navigateTo('全部文件');
  ab.waitMs(2000);
  var snap = ab.snapshot();
  // Look for file action dropdown
  var actionLink = ab.findRefByText(snap, '操作');
  if (actionLink) {
    ab.click(actionLink);
    ab.waitMs(500);
    snap = ab.snapshot();
    ab.screenshot('t10-15-action-menu');
    // Try to find "授予权限" in the dropdown
    var grantBtn = ab.findRefByText(snap, '授予权限');
    if (grantBtn) {
      ab.click(grantBtn);
      ab.waitMs(1000);
      ab.screenshot('t10-15-grant-modal');
    }
  }

  // T12.16 - 权限列表展示验证
  navigateTo('权限管理');
  ab.waitMs(2000);
  ab.screenshot('t10-16-permission-list-final');

  // T12.17 - 清理：撤销所有测试授权
  revokePathPermAPI({ agentId: 'agent-test-003' }, '/**/*.txt');
  ab.waitMs(500);
  revokePathPermAPI({ agentGroupId: 'group-test-001' }, '/Documents/**');
  ab.waitMs(500);
  revokePathPermAPI({ agentId: 'agent-ui-test' }, '/TestFolder/**');
  ab.waitMs(500);

  // Fallback: list and delete any remaining permissions via API
  ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/permissions', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          var chain = Promise.resolve();
          items.forEach(function(p) {
            chain = chain.then(function() {
              return fetch('/v1/disk/permissions', {
                method: 'DELETE',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                  agentId: p.agentId || undefined,
                  agentGroupId: p.agentGroupId || undefined,
                  resourceId: p.resourceId || undefined,
                  resType: p.resType || undefined,
                  resourcePath: p.resourcePath || undefined
                }),
                credentials: 'include'
              }).then(function(r) { return r.json(); }).catch(function() {});
            });
          });
          return chain.then(function() { return items.length; });
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
  ab.waitMs(2000);

  // Clean up files
  ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files?folderId=0', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          var chain = Promise.resolve();
          items.forEach(function(f) {
            chain = chain.then(function() {
              return fetch('/v1/disk/files/' + f.id, {
                method: 'DELETE', credentials: 'include'
              }).then(function(r) { return r.json(); });
            });
          });
          return chain.then(function() { return items.length; });
        })
        .catch(function(e) { return -1; });
    })()
  `);
  ab.waitMs(3000);

  // Clean recycle bin
  ab.evalStdin(`
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
  ab.waitMs(2000);
  ab.screenshot('t10-17-cleanup');

  ab.closeBrowser();
});
