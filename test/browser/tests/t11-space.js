const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');
const fs = require('fs');
const path = require('path');
const os = require('os');

function getSpaceAPI() {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/space', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: total=' + d.data.totalQuota + ' used=' + d.data.usedQuota;
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

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
          return 'OK: id=' + d.data.id;
        })
        .catch(function(e) { return 'FETCH ERROR: ' + e.message; });
    })()
  `);
}

function deleteFileViaAPI(fileId) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files/${fileId}', { method: 'DELETE', credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: deleted';
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

describe('T11: 存储空间显示', () => {
  ab.closeAll();
  ab.login('user001', 'test123');
  ab.waitMs(2000);

  // T11.1 - 获取当前空间用量
  const space1 = getSpaceAPI();
  ab.waitMs(1000);
  const hasSpace = space1.includes('OK:');
  assertCondition(hasSpace, 'T11.1: 获取空间用量', space1);

  const usedBefore = space1.match(/used=(\d+)/);
  const usedValue = usedBefore ? parseInt(usedBefore[1]) : 0;
  ab.screenshot('t11-01-space-before');

  // T11.2 - 上传文件后验证用量变化
  const tmpFile = path.join(os.tmpdir(), 'agentdisk-space-test.bin');
  fs.writeFileSync(tmpFile, Buffer.alloc(1024 * 100, 'x')); // 100KB

  const uploadResult = uploadViaAPI(tmpFile);
  ab.waitMs(3000);
  const uploadOk = uploadResult.includes('OK:');
  assertCondition(uploadOk, 'T11.2: 上传文件成功', uploadResult);

  const idMatch = uploadResult.match(/id=(\d+)/);
  const fileId = idMatch ? idMatch[1] : null;

  const space2 = getSpaceAPI();
  ab.waitMs(1000);
  const usedAfter = space2.match(/used=(\d+)/);
  const usedAfterValue = usedAfter ? parseInt(usedAfter[1]) : 0;
  const increased = usedAfterValue > usedValue;
  assertCondition(increased, 'T11.2b: 上传后用量增加', 'before=' + usedValue + ' after=' + usedAfterValue);
  ab.screenshot('t11-02-space-after-upload');

  // T11.3 - 删除文件后验证用量减少
  if (fileId) {
    deleteFileViaAPI(fileId);
    ab.waitMs(2000);
  }

  // 清理回收站中的记录
  const cleanRecycle = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/recycle', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          var matches = items.filter(function(r) { return r.resName === 'agentdisk-space-test.bin'; });
          return Promise.all(matches.map(function(m) {
            return fetch('/v1/disk/recycle', {
              method: 'DELETE',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ recycleId: m.id }),
              credentials: 'include'
            }).then(function(r) { return r.json(); });
          })).then(function() { return 'OK: cleaned ' + matches.length; });
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
  ab.waitMs(2000);

  const space3 = getSpaceAPI();
  ab.waitMs(1000);
  const usedAfterDel = space3.match(/used=(\d+)/);
  const usedAfterDelValue = usedAfterDel ? parseInt(usedAfterDel[1]) : 0;
  const decreased = usedAfterDelValue < usedAfterValue || usedAfterDelValue <= usedValue;
  assertCondition(decreased, 'T11.3: 删除后用量减少', 'before=' + usedValue + ' afterDel=' + usedAfterDelValue + ' clean=' + cleanRecycle);
  ab.screenshot('t11-03-space-after-delete');

  try { fs.unlinkSync(tmpFile); } catch {}
  ab.closeBrowser();
});
