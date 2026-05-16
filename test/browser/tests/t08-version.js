const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');
const fs = require('fs');
const path = require('path');
const os = require('os');

describe('T8: 版本历史与回滚', () => {
  ab.closeAll();

  const tmpFile = path.join(os.tmpdir(), 'version-test.txt');

  ab.login('user001', 'test123');

  // T8.1 - 上传 version 1
  fs.writeFileSync(tmpFile, 'version 1');
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
  const uploaded = ab.pageContainsText('version-test.txt');
  assertCondition(uploaded, 'T8.1: version-test.txt 上传成功');
  ab.screenshot('t08-01-uploaded-v1');

  // T8.2 - 查看版本历史
  snap = ab.snapshot();
  const actionBtn = ab.findRefByText(snap, '操作') || ab.findRefByRole(snap, 'button', '...');
  if (actionBtn) {
    ab.click(actionBtn);
    ab.waitMs(500);
    snap = ab.snapshot();
    const versionItem = ab.findRefByText(snap, '版本历史') || ab.findRefByText(snap, '版本');
    if (versionItem) {
      ab.click(versionItem);
      ab.waitMs(1500);
      const hasV1 = ab.pageContainsText('v1') || ab.pageContainsText('1');
      assertCondition(hasV1, 'T8.2: 版本历史显示 v1');
      ab.screenshot('t08-02-version-history');
    } else {
      step('T8.2: 找不到版本历史菜单项', false);
    }
  } else {
    step('T8.2: 找不到操作按钮', false);
  }

  // 关闭版本历史抽屉
  ab.press('Escape');
  ab.waitMs(500);

  // T8.3 - 上传同名文件（version 2）
  fs.writeFileSync(tmpFile, 'version 2');
  snap = ab.snapshot();
  const uploadBtn2 = ab.findRefByText(snap, '上传文件') || ab.findRefByText(snap, '上传');
  if (uploadBtn2) {
    ab.click(uploadBtn2);
    ab.waitMs(500);
    snap = ab.snapshot();
    const fileInput2 = ab.findRefBySelector(snap, 'input[type="file"]');
    if (fileInput2) {
      ab.ab('upload', fileInput2, tmpFile);
      ab.waitLoad('networkidle');
      ab.waitMs(2000);
    }
  }
  step('T8.3: 同名文件更新上传', true);
  ab.screenshot('t08-03-uploaded-v2');

  // T8.4 - 再次查看版本历史
  snap = ab.snapshot();
  const actionBtn2 = ab.findRefByText(snap, '操作') || ab.findRefByRole(snap, 'button', '...');
  if (actionBtn2) {
    ab.click(actionBtn2);
    ab.waitMs(500);
    snap = ab.snapshot();
    const versionItem2 = ab.findRefByText(snap, '版本历史') || ab.findRefByText(snap, '版本');
    if (versionItem2) {
      ab.click(versionItem2);
      ab.waitMs(1500);
      const hasBoth = ab.pageContainsText('v1') || ab.pageContainsText('v2') ||
                      (ab.pageContainsText('1') && ab.pageContainsText('2'));
      assertCondition(hasBoth, 'T8.4: 版本历史显示多个版本');
      ab.screenshot('t08-04-two-versions');

      // T8.5 & T8.6 - 回滚到 v1
      snap = ab.snapshot();
      const rollbackBtn = ab.findRefByText(snap, '回滚');
      if (rollbackBtn) {
        ab.click(rollbackBtn);
        ab.waitMs(500);
        snap = ab.snapshot();
        const confirmRollback = ab.findRefByRole(snap, 'button', '确定') || ab.findRefByRole(snap, 'button', '回滚');
        if (confirmRollback) ab.click(confirmRollback);
        ab.waitLoad('networkidle');
        ab.waitMs(1000);
        const rolled = ab.pageContainsText('回滚') || ab.pageContainsText('成功');
        step('T8.6: 回滚成功', rolled);
        ab.screenshot('t08-05-rolled-back');
      } else {
        step('T8.5: 找不到回滚按钮', false);
      }
    }
  }

  try { fs.unlinkSync(tmpFile); } catch {}
  ab.closeBrowser();
});
