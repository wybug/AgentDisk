const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T4: 文件预览', () => {
  ab.closeAll();
  ab.login('user001', 'test123');

  // T4.1 - 点击 md 文件名称预览
  let snap = ab.snapshot();
  const mdFile = ab.findRefByText(snap, 'test-upload.md');
  if (mdFile) {
    ab.click(mdFile);
    ab.waitLoad('networkidle');
    ab.waitMs(2000);
    ab.screenshot('t04-01-md-preview');

    const hasMdContent = ab.pageContainsText('Test Markdown') || ab.pageContainsText('Markdown');
    assertCondition(hasMdContent, 'T4.1: Markdown 预览正常渲染');

    // T4.2 - 返回文件列表
    ab.findAndClick('返回');
    ab.waitLoad('networkidle');
    ab.waitMs(1000);
    ab.screenshot('t04-02-back-to-list');
  } else {
    step('T4.1: 找不到 md 文件，跳过预览测试', false, '请先运行 T3 上传文件');
    ab.screenshot('t04-01-no-md-file');
    ab.closeBrowser();
    return;
  }

  // T4.3 - 点击 txt 文件预览
  snap = ab.snapshot();
  const txtFile = ab.findRefByText(snap, 'test-upload.txt');
  if (txtFile) {
    ab.click(txtFile);
    ab.waitLoad('networkidle');
    ab.waitMs(1500);
    const hasTxtContent = ab.pageContainsText('Hello') || ab.pageContainsText('AgentDisk');
    assertCondition(hasTxtContent, 'T4.3: 文本文件预览正常');
    ab.screenshot('t04-03-txt-preview');

    ab.findAndClick('返回');
    ab.waitLoad('networkidle');
    ab.waitMs(1000);
  } else {
    step('T4.3: txt 文件预览', false, '找不到 txt 文件');
  }

  ab.closeBrowser();
});
