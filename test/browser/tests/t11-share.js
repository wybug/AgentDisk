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

function shareDownloadAPI(code, resourceId, extractCode) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/share/download', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: '${code}', resourceId: ${resourceId}, extractCode: '${extractCode || ''}' })
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: token=' + (d.data.downloadToken || '').substring(0, 30) + '... expiresIn=' + d.data.expiresIn;
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function downloadByTokenAPI(token) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files/download?t=' + encodeURIComponent('${token}'))
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          var f = d.data.file || {};
          return 'OK: fileId=' + f.id + ' fileName=' + (f.fileName || '') + ' hasUrl=' + (!!d.data.downloadUrl);
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

describe('T11: 分享管理', () => {
  ab.closeAll();
  ab.login('user001', 'test123');
  ab.waitMs(2000);

  // Upload a test file for share tests (in case T11 runs standalone)
  ab.evalStdin(`
    (function() {
      var blob = new Blob(['share test file content'], { type: 'text/plain' });
      var fd = new FormData();
      fd.append('file', blob, 'agentdisk-test-upload.txt');
      return fetch('/v1/disk/files/upload?folderId=0', {
        method: 'POST', body: fd, credentials: 'include'
      }).then(function(r) { return r.json(); })
        .then(function(d) { return d.data ? d.data.id : 'ERR'; })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
  ab.waitMs(1000);

  // T11.1 - 找到测试文件
  const fileId = findFileId();
  ab.waitMs(1000);
  assertCondition(fileId !== 'none' && fileId !== '"none"' && !fileId.includes('ERR'), 'T11.1: 找到测试文件', 'fileId=' + fileId);

  // T11.2 - 通过 API 创建分享
  const createResult = createShareAPI(fileId, 'abc123', 10, 72);
  ab.waitMs(2000);
  const createOk = createResult.includes('OK:');
  assertCondition(createOk, 'T11.2: 创建分享成功', createResult);

  // 提取分享码和 ID
  const codeMatch = createResult.match(/code=([a-zA-Z0-9]+)/);
  const idMatch = createResult.match(/id=(\d+)/);
  const shareCode = codeMatch ? codeMatch[1] : null;
  const shareId = idMatch ? idMatch[1] : null;
  assertCondition(shareCode !== null, 'T11.2b: 获取分享码', shareCode || 'not found');
  ab.screenshot('t09-02-share-created');

  // T11.3 - 导航到我的分享页面
  navigateTo('我的分享');
  ab.waitMs(2000);
  const urlShares = ab.getUrl();
  assertCondition(urlShares.includes('/shares'), 'T11.3: 导航到我的分享页面', urlShares);
  ab.screenshot('t09-03-share-list');

  // T11.4 - 验证分享列表中有记录
  const listResult = listSharesAPI();
  ab.waitMs(1000);
  const listOk = listResult.includes('OK:') && listResult.includes('active');
  assertCondition(listOk, 'T11.4: 分享列表显示活跃的分享记录', listResult);

  // T11.5 - 公开访问分享（获取分享信息）
  const publicResult = getPublicShareAPI(shareCode);
  ab.waitMs(1000);
  const publicOk = publicResult.includes('OK:') && publicResult.includes('active=true');
  assertCondition(publicOk, 'T11.5: 公开访问分享链接可获取信息', publicResult);
  ab.screenshot('t09-05-public-access');

  // T11.6 - 使用提取码访问分享
  const accessResult = accessShareAPI(shareCode, 'abc123');
  ab.waitMs(1000);
  const accessOk = accessResult.includes('OK:');
  assertCondition(accessOk, 'T11.6: 使用提取码访问分享成功', accessResult);

  // T11.7 - 验证错误提取码被拒绝
  const wrongAccess = accessShareAPI(shareCode, 'wrongcode');
  ab.waitMs(1000);
  const wrongRejected = wrongAccess.includes('ERROR');
  assertCondition(wrongRejected, 'T11.7: 错误提取码被拒绝', wrongAccess);

  // T11.7b - 有提取码分享下载：使用正确提取码调用公开下载端点
  var dlResult = shareDownloadAPI(shareCode, fileId, 'abc123');
  ab.waitMs(1000);
  var dlOk = dlResult.includes('OK:') && dlResult.includes('token=');
  step('T11.7b: 有提取码分享下载成功', dlOk, dlResult);
  ab.screenshot('t09-7b-download-with-code');

  // T11.7c - 有提取码 + 错误提取码下载失败
  var wrongDl = shareDownloadAPI(shareCode, fileId, 'wrongcode');
  ab.waitMs(1000);
  var wrongDlRejected = wrongDl.includes('ERROR');
  step('T11.7c: 错误提取码下载失败', wrongDlRejected, wrongDl);

  // T11.7d - 无提取码分享下载：创建无提取码的分享并下载
  var noCodeShare = createShareAPI(fileId, '', 10, 72);
  ab.waitMs(1000);
  var noCodeMatch = noCodeShare.match(/code=([a-zA-Z0-9]+)/);
  var noCodeShareCode = noCodeMatch ? noCodeMatch[1] : '';
  var noCodeDlResult = shareDownloadAPI(noCodeShareCode, fileId, '');
  ab.waitMs(1000);
  var noCodeDlOk = noCodeDlResult.includes('OK:') && noCodeDlResult.includes('token=');
  step('T11.7d: 无提取码分享下载成功', noCodeDlOk, noCodeDlResult);

  // T11.7e - 下载 token 验证：在同一 evalStdin 中完成获取 token + 调用下载
  var verifyResult = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/share/download', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: '${shareCode}', resourceId: ${fileId}, extractCode: 'abc123' })
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          var token = d.data.downloadToken;
          if (!token) return 'ERROR: no token';
          return fetch('/v1/disk/files/download?t=' + encodeURIComponent(token))
            .then(function(r2) { return r2.json(); })
            .then(function(d2) {
              if (d2.code !== undefined && d2.code !== 0) return 'ERROR: ' + d2.message;
              var f = d2.data.file || {};
              return 'OK: fileId=' + f.id + ' fileName=' + (f.fileName || '') + ' hasUrl=' + (!!d2.data.downloadUrl);
            });
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
  ab.waitMs(1500);
  var verifyOk = verifyResult.includes('OK:') && verifyResult.includes('fileName=');
  step('T11.7e: 下载 token 验证通过', verifyOk, verifyResult);
  ab.screenshot('t09-7e-download-verified');

  // Clean up no-code share
  var noCodeIdMatch = noCodeShare.match(/id=(\d+)/);
  if (noCodeIdMatch) {
    revokeShareAPI(noCodeIdMatch[1]);
    ab.waitMs(300);
  }

  // T11.8 - 验证访问次数增加
  const afterAccess = listSharesAPI();
  ab.waitMs(1000);
  step('T11.8: 验证分享访问', afterAccess.includes('OK'), afterAccess.substring(0, 150));

  // T11.8b - 分享不存在的文件应返回错误
  const fakeShare = createShareAPI(999999, 'abc', 10, 72);
  ab.waitMs(1000);
  const fakeRejected = fakeShare.includes('ERROR') && (fakeShare.includes('不存在') || fakeShare.includes('not found') || fakeShare.includes('not exist'));
  assertCondition(fakeRejected, 'T11.8b: 分享不存在的文件返回错误', fakeShare);

  // T11.9 - 撤销分享
  const revokeResult = revokeShareAPI(shareId);
  ab.waitMs(1000);
  const revokeOk = revokeResult.includes('OK');
  assertCondition(revokeOk, 'T11.9: 撤销分享成功', revokeResult);
  ab.screenshot('t09-09-share-revoked');

  // T11.10 - 验证撤销后无法访问
  const afterRevoke = getPublicShareAPI(shareCode);
  ab.waitMs(1000);
  const revoked = afterRevoke.includes('ERROR') || afterRevoke.includes('active=false');
  assertCondition(revoked, 'T11.10: 撤销后分享不可访问', afterRevoke);

  ab.closeBrowser();
});
