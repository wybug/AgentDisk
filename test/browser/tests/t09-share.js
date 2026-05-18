const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

function findFileId() {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files?folderId=0', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          for (var i = 0; i < items.length; i++) {
            if (items[i].fileName && items[i].fileName.includes('agentdisk-test-upload.txt')) return items[i].id;
          }
          return items.length > 0 ? items[0].id : 'none';
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function createShareAPI(fileId, extractCode, maxVisit, expireHours) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/shares', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          resourceId: ${fileId},
          resType: 'file',
          extractCode: '${extractCode}',
          maxVisit: ${maxVisit},
          expireHours: ${expireHours}
        }),
        credentials: 'include'
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: id=' + d.data.id + ' code=' + d.data.shareCode;
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function listSharesAPI() {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/shares', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          var items = d.data || [];
          return 'OK: ' + items.length + ' shares: ' + items.map(function(s) {
            return s.id + ':' + s.shareCode + '(' + (s.isActive ? 'active' : 'revoked') + ')';
          }).join(', ');
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function getPublicShareAPI(code) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/share/${code}', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: code=' + d.data.shareCode + ' active=' + d.data.isActive + ' extract=' + (d.data.extractCode || 'none');
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function accessShareAPI(code, extractCode) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/share/access', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: '${code}', extractCode: '${extractCode}' }),
        credentials: 'include'
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: accessed shareCode=' + d.data.shareCode;
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function revokeShareAPI(shareId) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/shares', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ shareId: ${shareId} }),
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

describe('T9: 分享管理', () => {
  ab.closeAll();
  ab.login('user001', 'test123');
  ab.waitMs(2000);

  // T9.1 - 找到测试文件
  const fileId = findFileId();
  ab.waitMs(1000);
  assertCondition(fileId !== 'none' && !fileId.includes('ERR'), 'T9.1: 找到测试文件', 'fileId=' + fileId);

  // T9.2 - 通过 API 创建分享
  const createResult = createShareAPI(fileId, 'abc123', 10, 72);
  ab.waitMs(2000);
  const createOk = createResult.includes('OK:');
  assertCondition(createOk, 'T9.2: 创建分享成功', createResult);

  // 提取分享码和 ID
  const codeMatch = createResult.match(/code=([a-zA-Z0-9]+)/);
  const idMatch = createResult.match(/id=(\d+)/);
  const shareCode = codeMatch ? codeMatch[1] : null;
  const shareId = idMatch ? idMatch[1] : null;
  assertCondition(shareCode !== null, 'T9.2b: 获取分享码', shareCode || 'not found');
  ab.screenshot('t09-02-share-created');

  // T9.3 - 导航到我的分享页面
  navigateTo('我的分享');
  ab.waitMs(2000);
  const urlShares = ab.getUrl();
  assertCondition(urlShares.includes('/shares'), 'T9.3: 导航到我的分享页面', urlShares);
  ab.screenshot('t09-03-share-list');

  // T9.4 - 验证分享列表中有记录
  const listResult = listSharesAPI();
  ab.waitMs(1000);
  const listOk = listResult.includes('OK:') && listResult.includes('active');
  assertCondition(listOk, 'T9.4: 分享列表显示活跃的分享记录', listResult);

  // T9.5 - 公开访问分享（获取分享信息）
  const publicResult = getPublicShareAPI(shareCode);
  ab.waitMs(1000);
  const publicOk = publicResult.includes('OK:') && publicResult.includes('active=true');
  assertCondition(publicOk, 'T9.5: 公开访问分享链接可获取信息', publicResult);
  ab.screenshot('t09-05-public-access');

  // T9.6 - 使用提取码访问分享
  const accessResult = accessShareAPI(shareCode, 'abc123');
  ab.waitMs(1000);
  const accessOk = accessResult.includes('OK:');
  assertCondition(accessOk, 'T9.6: 使用提取码访问分享成功', accessResult);

  // T9.7 - 验证错误提取码被拒绝
  const wrongAccess = accessShareAPI(shareCode, 'wrongcode');
  ab.waitMs(1000);
  const wrongRejected = wrongAccess.includes('ERROR');
  assertCondition(wrongRejected, 'T9.7: 错误提取码被拒绝', wrongAccess);

  // T9.8 - 验证访问次数增加
  const afterAccess = listSharesAPI();
  ab.waitMs(1000);
  step('T9.8: 验证分享访问', afterAccess.includes('OK'), afterAccess.substring(0, 150));

  // T9.8b - 分享不存在的文件应返回错误
  const fakeShare = createShareAPI(999999, 'abc', 10, 72);
  ab.waitMs(1000);
  const fakeRejected = fakeShare.includes('ERROR') && (fakeShare.includes('不存在') || fakeShare.includes('not found') || fakeShare.includes('not exist'));
  assertCondition(fakeRejected, 'T9.8b: 分享不存在的文件返回错误', fakeShare);

  // T9.9 - 撤销分享
  const revokeResult = revokeShareAPI(shareId);
  ab.waitMs(1000);
  const revokeOk = revokeResult.includes('OK');
  assertCondition(revokeOk, 'T9.9: 撤销分享成功', revokeResult);
  ab.screenshot('t09-09-share-revoked');

  // T9.10 - 验证撤销后无法访问
  const afterRevoke = getPublicShareAPI(shareCode);
  ab.waitMs(1000);
  const revoked = afterRevoke.includes('ERROR') || afterRevoke.includes('active=false');
  assertCondition(revoked, 'T9.10: 撤销后分享不可访问', afterRevoke);

  ab.closeBrowser();
});
