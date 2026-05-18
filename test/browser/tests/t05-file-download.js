const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

function navigateToExplorer() {
  ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('全部文件')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(2000);
}

function clickFileAction(fileName, action) {
  return ab.evalStdin(`
    (function() {
      var rows = document.querySelectorAll('.ant-table-row');
      for (var i = 0; i < rows.length; i++) {
        if (rows[i].textContent.includes('${fileName}')) {
          var trigger = rows[i].querySelector('a');
          var allLinks = rows[i].querySelectorAll('a');
          for (var j = 0; j < allLinks.length; j++) {
            if (allLinks[j].textContent.trim() === '操作') { trigger = allLinks[j]; break; }
          }
          if (trigger) { trigger.click(); return 'clicked trigger'; }
          return 'no trigger';
        }
      }
      return 'row not found';
    })()
  `);
}

function getDownloadTokenViaAPI(fileId) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files/${fileId}/download-token', { method: 'POST', credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: token=' + d.data.downloadToken + ' expiresIn=' + d.data.expiresIn;
        })
        .catch(function(e) { return 'FETCH ERROR: ' + e.message; });
    })()
  `);
}

function listFilesViaAPI() {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files?folderId=0', { credentials: 'include' })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== undefined && d.code !== 0) return 'ERROR: ' + d.message;
          var items = d.data || [];
          return items.map(function(f) { return f.id + ':' + f.fileName; }).join(', ');
        })
        .catch(function(e) { return 'FETCH ERROR: ' + e.message; });
    })()
  `);
}

describe('T5: 文件下载', () => {
  ab.closeAll();
  ab.login('user001', 'test123');

  ab.waitMs(2000);
  navigateToExplorer();
  ab.screenshot('t05-00-explorer');

  // T5.1 - 列出文件，找到一个可下载的文件
  const fileList = listFilesViaAPI();
  ab.waitMs(1000);
  const hasFiles = fileList.includes('agentdisk-test-upload');
  assertCondition(hasFiles, 'T5.1: 找到已上传的测试文件', hasFiles ? fileList.substring(0, 200) : '请先运行 T3 上传文件');

  // 提取第一个文件ID
  const fileIdMatch = fileList.match(/(\d+):agentdisk-test-upload/);
  const fileId = fileIdMatch ? fileIdMatch[1] : null;
  assertCondition(fileId !== null, 'T5.1b: 获取文件 ID', fileId || 'not found');

  // T5.2 - 通过 API 获取下载令牌
  const tokenResult = getDownloadTokenViaAPI(fileId);
  ab.waitMs(2000);
  const tokenOk = tokenResult.includes('OK: token=');
  assertCondition(tokenOk, 'T5.2: 获取下载令牌成功', tokenResult.substring(0, 200));
  ab.screenshot('t05-02-download-token');

  // T5.3 - 验证下载 URL 可访问
  const tokenMatch = tokenResult.match(/token=([a-zA-Z0-9._-]+)/);
  if (tokenMatch) {
    const downloadUrl = '/v1/disk/files/download?token=' + tokenMatch[1];
    const downloadCheck = ab.evalStdin(`
      (function() {
        return fetch('${downloadUrl}', { credentials: 'include' })
          .then(function(r) {
            var ct = r.headers.get('content-type') || '';
            return 'status=' + r.status + ' content-type=' + ct;
          })
          .catch(function(e) { return 'FETCH ERROR: ' + e.message; });
      })()
    `);
    ab.waitMs(2000);
    const downloadOk = downloadCheck.includes('status=200');
    assertCondition(downloadOk, 'T5.3: 下载 URL 可访问', downloadCheck);
  } else {
    step('T5.3: 无法提取 token，跳过下载验证', false, tokenResult);
  }
  ab.screenshot('t05-03-download-url');

  // T5.4 - 通过 UI 点击操作菜单下载
  const uiFile = 'agentdisk-test-upload.txt';
  const clickResult = clickFileAction(uiFile, '下载');
  ab.waitMs(500);
  ab.screenshot('t05-04-action-menu');

  // 在下拉菜单中点击下载
  const menuClick = ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-dropdown-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('下载')) { items[i].click(); return 'clicked download'; }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(2000);
  step('T5.4: 通过 UI 触发下载操作', menuClick.includes('clicked'), menuClick);
  ab.screenshot('t05-04-download-triggered');

  ab.closeBrowser();
});
