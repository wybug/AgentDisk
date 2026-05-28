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
          return 'OK: ' + d.data.fileName + ' id=' + d.data.id;
        })
        .catch(function(e) { return 'FETCH ERROR: ' + e.message; });
    })()
  `);
}

function goBackToExplorer() {
  ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        var t = btns[i].textContent;
        if (t.includes('返回') || t.includes('文件列表')) { btns[i].click(); return 'clicked: ' + t; }
      }
      window.location.href = '/explorer';
      return 'navigated';
    })()
  `);
  ab.waitLoad('networkidle');
  ab.waitMs(1500);
}

// Click file link by name using JS — match first link (newest, since list is DESC)
function clickFileLink(fileName) {
  return ab.evalStdin(`
    (function() {
      var links = document.querySelectorAll('.ant-table-row a');
      for (var i = 0; i < links.length; i++) {
        if (links[i].textContent.includes('${fileName}')) { links[i].click(); return 'clicked link'; }
      }
      return 'not found';
    })()
  `);
}

describe('T06: 文件预览', () => {
  ab.closeAll();
  ab.login('user001', 'test123');

  // 确保在 explorer 页面
  ab.waitMs(2000);
  const currentUrl = ab.getUrl();
  if (!currentUrl.includes('/explorer')) {
    ab.jsClickElement('.ant-menu-item');
    ab.waitMs(2000);
  }

  // T06.1 - 点击 md 文件名称预览
  let snap = ab.snapshot();
  const mdFile = ab.findRefByText(snap, 'agentdisk-test-upload.md');
  if (mdFile) {
    ab.click(mdFile);
    ab.waitLoad('networkidle');
    ab.waitMs(2000);
    ab.screenshot('t04-01-md-preview');

    const hasMdContent = ab.pageContainsText('Markdown') || ab.pageContainsText('item');
    assertCondition(hasMdContent, 'T06.1: Markdown 预览正常渲染');

    // T06.2 - 返回文件列表
    goBackToExplorer();
    const backUrl = ab.getUrl();
    assertCondition(backUrl.includes('/explorer'), 'T06.2: 返回文件列表', backUrl);
    ab.screenshot('t04-02-back-to-list');
  } else {
    step('T06.1: 找不到 md 文件，跳过预览测试', false, '请先运行 T05 上传文件');
    ab.screenshot('t04-01-no-md-file');
    ab.closeBrowser();
    return;
  }

  // T06.3 - 点击 py 代码文件预览（用 JS 精确点击 link 元素）
  const pyClicked = clickFileLink('agentdisk-test-upload.py');
  ab.waitMs(3000);
  ab.screenshot('t04-03-py-preview');

  const pageText = ab.evalStdin('document.body.innerText');
  const hasCodeContent = pageText.includes('def') || pageText.includes('print') || pageText.includes('hello');
  assertCondition(hasCodeContent, 'T06.3: 代码文件预览正常', hasCodeContent ? '代码内容可见' : 'click=' + pyClicked + ' content=' + pageText.substring(0, 150));

  goBackToExplorer();

  // T06.5 - 点击 txt 文件预览
  const txtClicked = clickFileLink('agentdisk-test-upload.txt');
  ab.waitMs(3000);
  ab.screenshot('t04-05-txt-preview');

  const pageText2 = ab.evalStdin('document.body.innerText');
  const hasTxtContent = pageText2.includes('Hello') || pageText2.includes('AgentDisk');
  assertCondition(hasTxtContent, 'T06.5: 纯文本预览正常', hasTxtContent ? '文本内容可见' : 'click=' + txtClicked + ' content=' + pageText2.substring(0, 150));

  goBackToExplorer();

  // T06.6 - HTML 文件预览
  const htmlClicked = clickFileLink('agentdisk-test-upload.html');
  ab.waitMs(3000);
  ab.screenshot('t04-06-html-preview');

  const hasIframe = ab.evalStdin(`(function() {
    var iframe = document.querySelector('iframe');
    return iframe ? 'iframe found' : 'no iframe';
  })()`);

  const hasSecurityAlert = ab.pageContainsText('JavaScript') || ab.pageContainsText('禁用');
  assertCondition(hasIframe.includes('iframe found'), 'T06.6: HTML 预览使用 iframe 渲染', hasIframe);
  assertCondition(hasSecurityAlert, 'T06.7: HTML 预览显示安全提示', hasSecurityAlert ? '安全提示可见' : '安全提示不可见');

  goBackToExplorer();

  // T06.8 - 安全验证：上传恶意 HTML 并验证沙箱防护
  const evilHtml = path.join(os.tmpdir(), 'agentdisk-test-xss.html');
  fs.writeFileSync(evilHtml, '<!DOCTYPE html><html><body>\n<h1>Security Test</h1>\n<script>window.__xss_fired=true;<\/script>\n<img src=x onerror="window.__xss_img=true">\n<form action="https://evil.com"><input type="submit" value="phish"></form>\n<a href="https://evil.com" target="_blank">evil</a>\n</body></html>');

  const evilResult = uploadViaAPI(evilHtml);
  ab.waitMs(3000);

  ab.evalStdin('location.reload()');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');

  const evilClicked = clickFileLink('agentdisk-test-xss.html');
  ab.waitMs(3000);
  ab.screenshot('t04-07-security-test');

  // 验证 iframe sandbox 属性
  const sandboxCheck = ab.evalStdin(`(function() {
    var iframe = document.querySelector('iframe');
    if (!iframe) return 'no iframe';
    var sandbox = iframe.getAttribute('sandbox') || '';
    var hasAllowSameOrigin = sandbox.includes('allow-same-origin');
    var hasAllowForms = sandbox.includes('allow-forms');
    var hasAllowPopups = sandbox.includes('allow-popups');
    var hasAllowTopNav = sandbox.includes('allow-top-navigation');
    return 'sandbox=' + JSON.stringify(sandbox) +
      ' sameOrigin=' + hasAllowSameOrigin +
      ' forms=' + hasAllowForms +
      ' popups=' + hasAllowPopups +
      ' topNav=' + hasAllowTopNav;
  })()`);

  const noAllowSameOrigin = sandboxCheck.includes('sameOrigin=false');
  const noAllowForms = sandboxCheck.includes('forms=false');
  const noAllowPopups = sandboxCheck.includes('popups=false');
  const noAllowTopNav = sandboxCheck.includes('topNav=false');

  assertCondition(noAllowSameOrigin, 'T06.8: iframe 禁止 allow-same-origin', sandboxCheck);
  assertCondition(noAllowForms, 'T06.9: iframe 禁止 allow-forms', sandboxCheck);
  assertCondition(noAllowPopups, 'T06.10: iframe 禁止 allow-popups', sandboxCheck);
  assertCondition(noAllowTopNav, 'T06.11: iframe 禁止 allow-top-navigation', sandboxCheck);

  try { fs.unlinkSync(evilHtml); } catch {}

  goBackToExplorer();
  ab.screenshot('t04-08-html-preview-done');

  ab.closeBrowser();
});
