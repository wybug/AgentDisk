const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

function createFolderAPI(folderName) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/folders', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ parentId: 0, folderName: '${folderName}' }),
        credentials: 'include'
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: id=' + d.data.id + ' name=' + d.data.folderName;
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function listFilesAPI() {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files?folderId=0', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          return 'files: ' + items.length + ' - ' + items.map(function(f) { return f.fileName; }).join(', ');
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function listFoldersAPI() {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/folders?parentId=0', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          return 'folders: ' + items.length + ' - ' + items.map(function(f) { return f.folderName; }).join(', ');
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

describe('T15: 跨用户隔离', () => {
  ab.closeAll();

  // T15.1 - user001 登录，创建文件夹（使用时间戳避免重复）
  ab.login('user001', 'test123');
  ab.waitMs(2000);

  const folderName = 'user1-' + Date.now();
  const createResult = createFolderAPI(folderName);
  ab.waitMs(2000);
  const createOk = createResult.includes('OK:');
  assertCondition(createOk, 'T15.1: user001 创建文件夹', createResult);
  ab.screenshot('t13-01-user1-folder');

  // 验证文件夹存在
  const folders1 = listFoldersAPI();
  ab.waitMs(1000);
  const hasFolder = folders1.includes(folderName);
  assertCondition(hasFolder, 'T15.1b: user001 可见自己的文件夹', folders1);

  // T15.2 - 关闭 user001 会话
  ab.closeBrowser();

  // T15.3 - user002 登录
  ab.login('user002', 'test123');
  ab.waitMs(2000);

  // T15.4 - 验证 user002 看不到 user001 的数据
  const folders2 = listFoldersAPI();
  ab.waitMs(1000);
  const files2 = listFilesAPI();
  ab.waitMs(1000);

  const noFolder = !folders2.includes(folderName);
  assertCondition(noFolder, 'T15.4: user002 看不到 user001 的文件夹', folders2 + ' | ' + files2);
  ab.screenshot('t13-04-user2-isolated');

  // T15.5 - 清理：切回 user001 删除测试文件夹
  ab.closeBrowser();
  ab.login('user001', 'test123');
  ab.waitMs(2000);
  var cleanResult = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/folders?parentId=0', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = (d.data || []).filter(function(f) { return f.folderName === '${folderName}'; });
          if (items.length === 0) return 'OK: already gone';
          return fetch('/v1/disk/folders/' + items[0].id, {
            method: 'DELETE', credentials: 'include'
          }).then(function(r) { return r.json(); })
            .then(function(d2) { return d2.code === 0 ? 'OK: deleted' : 'ERROR'; });
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
  ab.waitMs(1500);
  step('T15.5: user001 清理测试文件夹', cleanResult.includes('OK'), cleanResult);

  // 清理回收站中 T15 产生的记录
  ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/recycle', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          return Promise.all(items.map(function(m) {
            return fetch('/v1/disk/recycle', {
              method: 'DELETE',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ recycleId: m.id }),
              credentials: 'include'
            }).then(function(r) { return r.json(); });
          })).then(function() { return items.length; });
        })
        .catch(function(e) { return -1; });
    })()
  `);
  ab.waitMs(2000);

  ab.closeBrowser();
});
