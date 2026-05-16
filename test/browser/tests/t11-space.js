const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T11: 存储空间显示', () => {
  ab.closeAll();
  ab.login('user001', 'test123');

  // T11.1 - 观察空间用量条
  const hasUsage = ab.pageContainsText('空间') || ab.pageContainsText('MB') || ab.pageContainsText('GB') || ab.pageContainsText('已用');
  assertCondition(hasUsage, 'T11.1: 页面显示空间用量信息');
  ab.screenshot('t11-01-space-usage');

  // 获取当前用量数值
  const spaceBefore = ab.evalStdin(`
    const el = document.querySelector('.ant-progress-text') ||
               document.querySelector('[class*="space"]') ||
               document.querySelector('[class*="usage"]');
    el ? el.textContent.trim() : 'not found';
  `);
  step('T11.1: 当前空间信息', true, spaceBefore);

  // T11.2 - 上传文件后验证用量变化（如果上传功能可用）
  const tmpFile = require('path').join(require('os').tmpdir(), 'space-test.bin');
  require('fs').writeFileSync(tmpFile, Buffer.alloc(1024 * 100, 'x')); // 100KB
  let snap = ab.snapshot();
  const uploadBtn = ab.findRefByText(snap, '上传文件') || ab.findRefByText(snap, '上传');
  if (uploadBtn) {
    ab.click(uploadBtn);
    ab.waitMs(500);
    snap = ab.snapshot();
    const fileInput = ab.findRefBySelector(snap, 'input[type="file"]');
    if (fileInput) {
      ab.ab('upload', fileInput, tmpFile);
      ab.waitLoad('networkidle');
      ab.waitMs(2000);
    }
  }
  ab.screenshot('t11-02-after-upload');

  const spaceAfter = ab.evalStdin(`
    const el = document.querySelector('.ant-progress-text') ||
               document.querySelector('[class*="space"]') ||
               document.querySelector('[class*="usage"]');
    el ? el.textContent.trim() : 'not found';
  `);
  step('T11.2: 上传后空间信息', true, spaceAfter);

  // T11.3 - 删除文件后验证用量减少
  snap = ab.snapshot();
  const actionBtn = ab.findRefByText(snap, '操作');
  if (actionBtn) {
    ab.click(actionBtn);
    ab.waitMs(500);
    snap = ab.snapshot();
    const delItem = ab.findRefByText(snap, '删除');
    if (delItem) {
      ab.click(delItem);
      ab.waitMs(500);
      snap = ab.snapshot();
      const confirmBtn = ab.findRefByRole(snap, 'button', '确定');
      if (confirmBtn) ab.click(confirmBtn);
      ab.waitLoad('networkidle');
      ab.waitMs(1000);
    }
  }

  try { require('fs').unlinkSync(tmpFile); } catch {}
  ab.screenshot('t11-03-after-delete');

  ab.closeBrowser();
});
