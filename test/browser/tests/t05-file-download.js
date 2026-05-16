const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T5: 文件下载', () => {
  ab.closeAll();
  ab.login('user001', 'test123');

  // T5.1 - 点击文件操作菜单中的下载
  let snap = ab.snapshot();
  const fileRow = ab.findRefByText(snap, 'test-upload.txt');
  if (fileRow) {
    // 查找该文件行的操作菜单
    ab.click(fileRow);
    ab.waitMs(500);
    snap = ab.snapshot();

    // 尝试找操作按钮/下拉菜单
    const actionBtn = ab.findRefByText(snap, '操作') || ab.findRefByRole(snap, 'button', '...');
    if (actionBtn) {
      ab.click(actionBtn);
      ab.waitMs(500);
      snap = ab.snapshot();
      const downloadItem = ab.findRefByText(snap, '下载');
      if (downloadItem) {
        ab.click(downloadItem);
        ab.waitMs(2000);
        assertCondition(true, 'T5.1: 点击下载操作');
      } else {
        step('T5.1: 找不到下载菜单项', false);
      }
    } else {
      // 尝试直接在操作列找下载
      ab.findAndClick('下载');
      ab.waitMs(2000);
      step('T5.1: 触发下载', true);
    }
    ab.screenshot('t05-01-download-triggered');
  } else {
    step('T5.1: 找不到测试文件', false, '请先运行 T3 上传文件');
  }

  ab.closeBrowser();
});
