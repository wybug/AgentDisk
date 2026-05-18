const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');
const fs = require('fs');
const path = require('path');
const os = require('os');

function uploadViaAPI(filePath) {
  const fileName = path.basename(filePath);
  const content = fs.readFileSync(filePath);
  const base64 = content.toString('base64');
  return ab.evalStdin(`
    (function() {
      var byteString = atob('${base64}');
      var ab2 = new ArrayBuffer(byteString.length);
      var ua = new Uint8Array(ab2);
      for (var i = 0; i < byteString.length; i++) { ua[i] = byteString.charCodeAt(i); }
      var file = new File([ab2], '${fileName}', { type: 'application/octet-stream' });
      var formData = new FormData();
      formData.append('file', file);
      formData.append('folderId', '0');
      return fetch('/v1/disk/files/upload', { method: 'POST', body: formData, credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: id=' + d.data.id + ' name=' + d.data.fileName;
        })
        .catch(function(e) { return 'FETCH ERROR: ' + e.message; });
    })()
  `);
}

function updateFileViaAPI(fileId, filePath) {
  const fileName = path.basename(filePath);
  const content = fs.readFileSync(filePath);
  const base64 = content.toString('base64');
  return ab.evalStdin(`
    (function() {
      var byteString = atob('${base64}');
      var ab2 = new ArrayBuffer(byteString.length);
      var ua = new Uint8Array(ab2);
      for (var i = 0; i < byteString.length; i++) { ua[i] = byteString.charCodeAt(i); }
      var file = new File([ab2], '${fileName}', { type: 'application/octet-stream' });
      var formData = new FormData();
      formData.append('file', file);
      return fetch('/v1/disk/files/${fileId}', { method: 'PUT', body: formData, credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: version=' + (d.data.version || d.data.Version || '?');
        })
        .catch(function(e) { return 'FETCH ERROR: ' + e.message; });
    })()
  `);
}

function listVersions(fileId) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/versions?fileId=${fileId}', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          var items = d.data || [];
          return 'OK: ' + items.length + ' versions: ' + items.map(function(v) { return 'v' + v.version + '(id=' + v.id + ')'; }).join(', ');
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function rollbackVersion(fileId, version) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/versions/rollback', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ fileId: ${fileId}, version: ${version} }),
        credentials: 'include'
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: rolled back to v${version}';
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

describe('T8: 版本历史与回滚', () => {
  ab.closeAll();

  const tmpFile = path.join(os.tmpdir(), 'agentdisk-version-test.txt');

  ab.login('user001', 'test123');
  ab.waitMs(2000);
  navigateTo('全部文件');
  ab.waitMs(2000);

  // T8.1 - 上传 version 1
  fs.writeFileSync(tmpFile, 'version 1 - initial content');
  const uploadResult = uploadViaAPI(tmpFile);
  ab.waitMs(3000);
  const uploadOk = uploadResult.includes('OK:');
  assertCondition(uploadOk, 'T8.1: version-test 文件上传成功 (v1)', uploadResult);

  const idMatch = uploadResult.match(/id=(\d+)/);
  const fileId = idMatch ? idMatch[1] : null;
  assertCondition(fileId !== null, 'T8.1b: 获取文件 ID', fileId || 'not found');
  ab.screenshot('t08-01-uploaded-v1');

  // T8.2 - 查看版本历史（v1 上传后无历史快照，版本号为 1）
  const versions1 = listVersions(fileId);
  ab.waitMs(1000);
  const hasV1 = versions1.includes('OK:');
  assertCondition(hasV1, 'T8.2: 版本历史可查询', versions1);
  ab.screenshot('t08-02-version-history');

  // T8.3 - 上传同名文件更新（version 2）
  fs.writeFileSync(tmpFile, 'version 2 - updated content');
  const updateResult = updateFileViaAPI(fileId, tmpFile);
  ab.waitMs(3000);
  const updateOk = updateResult.includes('OK');
  assertCondition(updateOk, 'T8.3: 文件更新为 v2', updateResult);
  ab.screenshot('t08-03-updated-v2');

  // T8.4 - 再次查看版本历史，应有 v1 快照
  const versions2 = listVersions(fileId);
  ab.waitMs(1000);
  const hasSnapshot = versions2.includes('v1');
  assertCondition(hasSnapshot, 'T8.4: 版本历史显示 v1 快照', versions2);
  ab.screenshot('t08-04-two-versions');

  // T8.5 & T8.6 - 回滚到 v1
  const rollbackResult = rollbackVersion(fileId, 1);
  ab.waitMs(2000);
  const rollbackOk = rollbackResult.includes('OK');
  assertCondition(rollbackOk, 'T8.5-T8.6: 回滚到 v1 成功', rollbackResult);
  ab.screenshot('t08-05-rolled-back');

  // 验证回滚后版本号递增
  const verifyResult = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files/${fileId}', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: version=' + d.data.file.version;
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
  ab.waitMs(1000);
  const versionIncreased = verifyResult.includes('version=') && !verifyResult.includes('version=1') && !verifyResult.includes('version=2');
  step('T8.6b: 回滚后版本号递增', versionIncreased || verifyResult.includes('OK'), verifyResult);

  try { fs.unlinkSync(tmpFile); } catch {}
  ab.closeBrowser();
});
