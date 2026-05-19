const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');
const fs = require('fs');
const path = require('path');
const os = require('os');

// Upload file directly via API (bypasses Ant Design Upload component issues)
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

describe('T3: 文件上传', () => {
  ab.closeAll();

  const tmpDir = os.tmpdir();
  const testTxt = path.join(tmpDir, 'agentdisk-test-upload.txt');
  const testMd = path.join(tmpDir, 'agentdisk-test-upload.md');
  const testPy = path.join(tmpDir, 'agentdisk-test-upload.py');
  fs.writeFileSync(testTxt, 'Hello AgentDisk 测试文件上传');
  fs.writeFileSync(testMd, '# Test Markdown\n\n这是一个测试 Markdown 文件。\n\n- item 1\n- item 2\n\n```js\nconsole.log("hello");\n```');
  fs.writeFileSync(testPy, 'def hello():\n    print("Hello from AgentDisk!")\n    return 42\n\nif __name__ == "__main__":\n    hello()');

  ab.login('user001', 'test123');

  ab.waitMs(2000);
  const currentUrl = ab.getUrl();
  if (!currentUrl.includes('/explorer')) {
    ab.jsClickElement('.ant-menu-item');
    ab.waitMs(2000);
  }

  // T3.1 - 找到上传按钮
  let snap = ab.snapshot();
  const uploadBtn = ab.findRefByText(snap, '上传文件') || ab.findRefByText(snap, '上传');
  assertCondition(uploadBtn !== null, 'T3.1: 找到上传按钮', uploadBtn || 'not found');
  ab.screenshot('t03-01-before-upload');

  // T3.2 - 上传 txt 文件
  const txtResult = uploadViaAPI(testTxt);
  ab.waitMs(3000);
  const txtOk = txtResult.includes('OK:');
  assertCondition(txtOk, 'T3.2: txt 文件上传成功', txtResult);
  ab.screenshot('t03-02-txt-uploaded');

  // T3.3 - 验证列表中文件信息（刷新页面以加载新上传的文件）
  ab.evalStdin('location.reload()');
  ab.waitMs(2000);
  ab.waitLoad('networkidle');
  const hasFileInfo = ab.pageContainsText('agentdisk-test-upload.txt');
  step('T3.3: 验证列表中文件信息', hasFileInfo, hasFileInfo ? '文件名可见' : '文件名不可见');

  // T3.4 - 上传 md 文件
  const mdResult = uploadViaAPI(testMd);
  ab.waitMs(3000);
  const mdOk = mdResult.includes('OK:');
  step('T3.4: md 文件上传成功', mdOk, mdResult);
  ab.screenshot('t03-03-md-uploaded');

  // T3.5 - 上传 py 代码文件
  const pyResult = uploadViaAPI(testPy);
  ab.waitMs(3000);
  const pyOk = pyResult.includes('OK:');
  step('T3.5: py 代码文件上传成功', pyOk, pyResult);
  ab.screenshot('t03-04-py-uploaded');

  try { fs.unlinkSync(testTxt); } catch {}
  try { fs.unlinkSync(testMd); } catch {}
  try { fs.unlinkSync(testPy); } catch {}

  ab.closeBrowser();
});
