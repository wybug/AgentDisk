const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T17: 最终数据验证', () => {
  ab.closeAll();
  ab.login('user001', 'test123');
  ab.waitMs(2000);

  // T17.1 - 检查回收站是否为空
  var recycleCheck = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/recycle', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) { return (d.data || []).length; })
        .catch(function(e) { return -1; });
    })()
  `);
  ab.waitMs(1000);
  var recycleOk = Number(recycleCheck) === 0;
  assertCondition(recycleOk, 'T17.1: 回收站已清空', 'count=' + recycleCheck);

  // T17.2 - 检查分享列表是否为空（活跃分享）
  var shareCheck = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/shares', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var items = d.data || [];
          var active = items.filter(function(s) { return s.isActive === true; });
          return active.length;
        })
        .catch(function(e) { return -1; });
    })()
  `);
  ab.waitMs(1000);
  var shareOk = Number(shareCheck) === 0;
  assertCondition(shareOk, 'T17.2: 分享列表已清空', 'count=' + shareCheck);

  // T17.3 - 检查权限列表是否为空
  var permCheck = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/permissions', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) { return (d.data || []).length; })
        .catch(function(e) { return -1; });
    })()
  `);
  ab.waitMs(1000);
  var permOk = Number(permCheck) === 0;
  assertCondition(permOk, 'T17.3: 权限列表已清空', 'count=' + permCheck);

  // T17.4 - 检查文件列表是否为空
  var fileCheck = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files?folderId=0', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) { return (d.data || []).length; })
        .catch(function(e) { return -1; });
    })()
  `);
  ab.waitMs(1000);
  var fileOk = Number(fileCheck) === 0;
  assertCondition(fileOk, 'T17.4: 文件列表已清空', 'count=' + fileCheck);

  // T17.5 - 检查文件夹列表是否为空
  var folderCheck = ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/folders?parentId=0', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) { return (d.data || []).length; })
        .catch(function(e) { return -1; });
    })()
  `);
  ab.waitMs(1000);
  var folderOk = Number(folderCheck) === 0;
  assertCondition(folderOk, 'T17.5: 文件夹列表已清空', 'count=' + folderCheck);

  var allClean = recycleOk && shareOk && permOk && fileOk && folderOk;
  step('T17.6: 数据污染检查总结', allClean,
    'recycle=' + recycleCheck + ' shares=' + shareCheck + ' perms=' + permCheck + ' files=' + fileCheck + ' folders=' + folderCheck);
  ab.screenshot('t15-final-verify');

  ab.closeBrowser();
});
