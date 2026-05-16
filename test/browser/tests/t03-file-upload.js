const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');
const fs = require('fs');
const path = require('path');
const os = require('os');

describe('T3: 文件上传', () => {
  ab.closeAll();

  // 创建临时测试文件
  const tmpDir = os.tmpdir();
  const testTxt = path.join(tmpDir, 'test-upload.txt');
  const testMd = path.join(tmpDir, 'test-upload.md');
  fs.writeFileSync(testTxt, 'Hello AgentDisk 测试文件上传');
  fs.writeFileSync(testMd, '# Test Markdown\n\n这是一个测试 Markdown 文件。\n\n- item 1\n- item 2\n\n```js\nconsole.log("hello");\n```');

  ab.login('user001', 'test123');

  // T3.1 - 上传 txt 文件
  let snap = ab.snapshot();
  const uploadBtn = ab.findRefByText(snap, '上传文件') || ab.findRefByText(snap, '上传');
  assertCondition(uploadBtn !== null, 'T3.1: 找到上传按钮', uploadBtn || 'not found');

  if (uploadBtn) {
    ab.click(uploadBtn);
    ab.waitMs(500);
    ab.screenshot('t03-01-upload-dialog');

    snap = ab.snapshot();
    const fileInput = ab.findRefBySelector(snap, 'input[type="file"]');
    if (fileInput) {
      ab.ab('upload', fileInput, testTxt);
    } else {
      ab.abQuiet('find', 'text', '"点击或拖拽"', 'click');
      ab.waitMs(500);
      snap = ab.snapshot();
      const fInput = ab.findRefBySelector(snap, 'input[type="file"]');
      if (fInput) ab.ab('upload', fInput, testTxt);
    }
    ab.waitLoad('networkidle');
    ab.waitMs(2000);
  }

  // T3.2 - 验证上传成功
  const uploaded = ab.pageContainsText('test-upload.txt') || ab.pageContainsText('上传成功');
  assertCondition(uploaded, 'T3.2: txt 文件上传成功');
  ab.screenshot('t03-02-txt-uploaded');

  // T3.3 - 验证列表文件信息
  step('T3.3: 验证列表中文件信息', ab.pageContainsText('test-upload'));

  // T3.4 - 上传 md 文件
  snap = ab.snapshot();
  const uploadBtn2 = ab.findRefByText(snap, '上传文件') || ab.findRefByText(snap, '上传');
  if (uploadBtn2) {
    ab.click(uploadBtn2);
    ab.waitMs(500);
    snap = ab.snapshot();
    const fileInput2 = ab.findRefBySelector(snap, 'input[type="file"]');
    if (fileInput2) {
      ab.ab('upload', fileInput2, testMd);
    }
    ab.waitLoad('networkidle');
    ab.waitMs(2000);
  }
  const mdUploaded = ab.pageContainsText('test-upload.md');
  step('T3.4: md 文件上传成功', mdUploaded);
  ab.screenshot('t03-03-md-uploaded');

  // 清理临时文件
  try { fs.unlinkSync(testTxt); } catch {}
  try { fs.unlinkSync(testMd); } catch {}

  ab.closeBrowser();
});
