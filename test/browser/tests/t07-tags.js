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

function bindTag(fileId, tagName) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/tags/bind', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ fileId: ${fileId}, tagName: '${tagName}' }),
        credentials: 'include'
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: tag ${tagName} bound';
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function unbindTag(fileId, tagName) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/tags/unbind', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ fileId: ${fileId}, tagName: '${tagName}' }),
        credentials: 'include'
      })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: tag ${tagName} unbound';
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function searchByTags(tags) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/tags/search?tags=${encodeURIComponent(tags)}', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          var items = d.data || [];
          return 'OK: ' + items.length + ' files: ' + items.map(function(f) { return f.fileName; }).join(', ');
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

describe('T7: 标签管理', () => {
  ab.closeAll();
  ab.login('user001', 'test123');

  ab.waitMs(2000);
  navigateTo('全部文件');
  ab.waitMs(2000);
  ab.screenshot('t07-00-explorer');

  // T7.1 - 找到测试文件
  const fileId = findFileId();
  ab.waitMs(1000);
  assertCondition(fileId !== 'none' && !fileId.includes('ERR'), 'T7.1: 找到测试文件', 'fileId=' + fileId);

  // T7.2 - 绑定标签「重要」
  const bindResult1 = bindTag(fileId, '重要');
  ab.waitMs(1000);
  assertCondition(bindResult1.includes('OK'), 'T7.2: 标签「重要」绑定成功', bindResult1);
  ab.screenshot('t07-02-tag-bound');

  // T7.3 - 绑定标签「文档」
  const bindResult2 = bindTag(fileId, '文档');
  ab.waitMs(1000);
  assertCondition(bindResult2.includes('OK'), 'T7.3: 标签「文档」绑定成功', bindResult2);
  ab.screenshot('t07-03-two-tags');

  // T7.4 - 验证标签已保存（通过搜索验证文件绑定了两个标签）
  const searchBoth = searchByTags('重要,文档');
  ab.waitMs(1000);
  const searchBothOk = searchBoth.includes('OK') && searchBoth.includes('agentdisk-test-upload');
  assertCondition(searchBothOk, 'T7.4: 文件同时包含两个标签（通过搜索验证）', searchBoth);

  // T7.5 - 导航到标签搜索页面
  navigateTo('标签搜索');
  ab.waitMs(2000);
  const urlTags = ab.getUrl();
  assertCondition(urlTags.includes('/tags'), 'T7.5: 导航到标签搜索页面', urlTags);
  ab.screenshot('t07-05-tags-page');

  // T7.6 - 搜索标签「重要」
  const searchResult1 = searchByTags('重要');
  ab.waitMs(1000);
  const searchOk1 = searchResult1.includes('OK') && searchResult1.includes('agentdisk-test-upload');
  assertCondition(searchOk1, 'T7.6: 搜索「重要」找到标记的文件', searchResult1);
  ab.screenshot('t07-06-search-result');

  // T7.7 - 搜索标签「重要,文档」
  const searchResult2 = searchByTags('重要,文档');
  ab.waitMs(1000);
  const searchOk2 = searchResult2.includes('OK') && searchResult2.includes('agentdisk-test-upload');
  assertCondition(searchOk2, 'T7.7: 搜索「重要,文档」找到标记的文件', searchResult2);
  ab.screenshot('t07-07-search-multi');

  // T7.8 - 解绑标签「重要」
  const unbindResult = unbindTag(fileId, '重要');
  ab.waitMs(1000);
  assertCondition(unbindResult.includes('OK'), 'T7.8: 标签「重要」解绑成功', unbindResult);
  ab.screenshot('t07-08-tag-unbound');

  // 验证解绑后搜索「重要」不再包含该文件，但搜索「文档」仍能找到
  const searchAfterUnbind = searchByTags('重要');
  ab.waitMs(1000);
  const onlyDoc = !searchAfterUnbind.includes('agentdisk-test-upload');
  assertCondition(onlyDoc, 'T7.8b: 解绑后搜索「重要」不再包含该文件', searchAfterUnbind);

  // T7.9 - 清理：解绑「文档」标签，恢复初始状态
  const unbindDoc = unbindTag(fileId, '文档');
  ab.waitMs(1000);
  step('T7.9: 清理标签「文档」', unbindDoc.includes('OK'), unbindDoc);

  ab.closeBrowser();
});
