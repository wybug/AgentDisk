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

function deleteFileViaAPI(fileId) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files/${fileId}', { method: 'DELETE', credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: deleted';
        })
        .catch(function(e) { return 'FETCH ERROR: ' + e.message; });
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

function invalidateQueriesAndNavigate(page) {
  // Force React Query to refetch by navigating away and back
  navigateTo(page === '全部文件' ? '回收站' : '全部文件');
  ab.waitMs(1000);
  navigateTo(page);
  ab.waitMs(2000);
}

describe('T6: 文件删除与回收站', () => {
  ab.closeAll();

  const tmpFile = path.join(os.tmpdir(), 'agentdisk-test-recycle.txt');
  fs.writeFileSync(tmpFile, 'This file will be deleted and restored for recycle bin test');

  ab.login('user001', 'test123');

  ab.waitMs(2000);
  navigateTo('全部文件');
  ab.waitMs(2000);

  // T6.0 - 上传测试文件
  const uploadResult = uploadViaAPI(tmpFile);
  ab.waitMs(3000);
  const uploadOk = uploadResult.includes('OK:');
  assertCondition(uploadOk, 'T6.0: 上传测试文件', uploadResult);

  const idMatch = uploadResult.match(/id=(\d+)/);
  const fileId = idMatch ? idMatch[1] : null;
  assertCondition(fileId !== null, 'T6.0b: 获取文件 ID', fileId || 'not found');

  // 导航离开再回来刷新列表
  invalidateQueriesAndNavigate('全部文件');
  ab.screenshot('t06-00-uploaded');

  // T6.1 & T6.2 - 通过 API 删除文件
  const deleteResult = deleteFileViaAPI(fileId);
  ab.waitMs(2000);
  const deleteOk = deleteResult.includes('OK');
  assertCondition(deleteOk, 'T6.1-T6.2: 删除文件', deleteResult);

  // 通过 API 验证文件已从列表消失（soft delete）
  const verifyDeleted = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files?folderId=0', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          var found = items.filter(function(f) { return f.id === ${fileId}; });
          return found.length === 0 ? 'OK: gone' : 'FAIL: still visible';
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
  ab.waitMs(1000);
  assertCondition(verifyDeleted.includes('OK'), 'T6.2b: 文件从列表消失', verifyDeleted);
  ab.screenshot('t06-02-after-delete');

  // T6.3 - 进入回收站
  navigateTo('回收站');
  ab.waitMs(2000);

  let urlRecycle = ab.getUrl();
  let inRecycle = urlRecycle.includes('/recycle');
  assertCondition(inRecycle, 'T6.3: 进入回收站页面', urlRecycle);
  ab.screenshot('t06-03-recycle-bin');

  const recycleHasFile = ab.pageContainsText('agentdisk-test-recycle.txt');
  assertCondition(recycleHasFile, 'T6.3b: 回收站中可见被删除的文件', recycleHasFile ? 'ok' : '文件不在回收站');

  // T6.4 - 恢复文件
  if (recycleHasFile) {
    const restoreResult = ab.evalStdin(`
      (function() {
        var rows = document.querySelectorAll('.ant-table-row');
        for (var i = 0; i < rows.length; i++) {
          if (rows[i].textContent.includes('agentdisk-test-recycle.txt')) {
            var btns = rows[i].querySelectorAll('button');
            for (var j = 0; j < btns.length; j++) {
              if (btns[j].textContent.includes('恢复')) { btns[j].click(); return 'clicked restore'; }
            }
            return 'no restore btn';
          }
        }
        return 'row not found';
      })()
    `);
    ab.waitMs(2000);
    ab.waitLoad('networkidle');
    step('T6.4: 点击恢复按钮', restoreResult.includes('clicked'), restoreResult);
    ab.screenshot('t06-04-restored');
  } else {
    step('T6.4: 跳过恢复（文件不在回收站）', false, '前置条件不满足');
  }

  // T6.5 - 返回文件列表验证恢复
  navigateTo('全部文件');
  ab.waitMs(2000);

  const backInList = ab.pageContainsText('agentdisk-test-recycle.txt');
  assertCondition(backInList, 'T6.5: 恢复的文件重新出现在列表中', backInList ? 'ok' : '文件未恢复');
  ab.screenshot('t06-05-back-in-list');

  // T6.6 - 再次删除文件
  const deleteResult2 = deleteFileViaAPI(fileId);
  ab.waitMs(2000);
  step('T6.6: 再次删除文件', deleteResult2.includes('OK'), deleteResult2);
  ab.screenshot('t06-06-deleted-again');

  // T6.7 - 彻底删除（通过 API 获取回收站记录）
  navigateTo('回收站');
  ab.waitMs(2000);

  const recycleCheck2 = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/recycle', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          var matches = items.filter(function(r) { return r.resName === 'agentdisk-test-recycle.txt'; });
          return matches.length > 0 ? 'found:' + matches[0].id : 'not found';
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
  ab.waitMs(1000);
  const recycleHasFile2 = recycleCheck2.includes('found:');
  if (recycleHasFile2) {
    // 获取所有匹配的回收站记录并逐个删除
    const permDelResult = ab.evalStdin(`
      (function() {
        return fetch('/v1/disk/recycle', { credentials: 'include' })
          .then(function(r) { return r.json(); })
          .then(function(d) {
            var items = d.data || [];
            var matches = items.filter(function(r) { return r.resName === 'agentdisk-test-recycle.txt'; });
            if (matches.length === 0) return 'none found';
            var ids = matches.map(function(m) { return m.id; });
            return Promise.all(ids.map(function(rid) {
              return fetch('/v1/disk/recycle', {
                method: 'DELETE',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ recycleId: rid }),
                credentials: 'include'
              }).then(function(r) { return r.json(); });
            })).then(function(results) {
              return 'OK: deleted ' + ids.length + ' records: ' + ids.join(',');
            });
          })
          .catch(function(e) { return 'ERR: ' + e.message; });
      })()
    `);
    ab.waitMs(2000);
    step('T6.7: 彻底删除', permDelResult.includes('OK'), permDelResult);
  } else {
    step('T6.7: 文件不在回收站，跳过彻底删除', false, '前置条件不满足');
  }
  ab.screenshot('t06-07-permanent-deleted');

  // 通过 API 验证回收站已清空
  const verifyPermDel = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/recycle', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          var found = items.filter(function(r) { return r.resName === 'agentdisk-test-recycle.txt'; });
          return found.length === 0 ? 'OK: gone from recycle' : 'FAIL: still in recycle (' + found.length + ')';
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
  ab.waitMs(1000);
  assertCondition(verifyPermDel.includes('OK'), 'T6.7b: 文件已从回收站彻底删除', verifyPermDel);
  ab.screenshot('t06-08-recycle-empty');

  try { fs.unlinkSync(tmpFile); } catch {}
  ab.closeBrowser();
});
