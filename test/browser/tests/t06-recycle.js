const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');
const fs = require('fs');
const path = require('path');
const os = require('os');

describe('T6: 文件删除与回收站', () => {
  ab.closeAll();

  // 创建一个用于测试删除的文件
  const tmpFile = path.join(os.tmpdir(), 'test-delete.txt');
  fs.writeFileSync(tmpFile, 'This file will be deleted for recycle bin test');

  ab.login('user001', 'test123');

  // 先上传一个文件用于删除测试
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

  // T6.1 - 点击文件的删除操作
  snap = ab.snapshot();
  const deleteFile = ab.findRefByText(snap, 'test-delete.txt');
  if (!deleteFile) {
    step('T6.1: 找不到测试文件', false, '上传可能失败');
    ab.closeBrowser();
    return;
  }
  ab.screenshot('t06-01-before-delete');

  // 查找操作菜单中的删除
  const actionBtn = ab.findRefByText(snap, '操作') || ab.findRefByRole(snap, 'button', '...');
  if (actionBtn) {
    ab.click(actionBtn);
    ab.waitMs(500);
    snap = ab.snapshot();
    const deleteItem = ab.findRefByText(snap, '删除');
    if (deleteItem) ab.click(deleteItem);
  } else {
    ab.findAndClick('删除');
  }
  ab.waitMs(1000);
  ab.screenshot('t06-02-delete-confirm');

  // T6.2 - 确认删除
  snap = ab.snapshot();
  const confirmBtn = ab.findRefByRole(snap, 'button', '确定') || ab.findRefByRole(snap, 'button', '删除');
  if (confirmBtn) ab.click(confirmBtn);
  else ab.findAndClick('确定');
  ab.waitLoad('networkidle');
  ab.waitMs(1000);

  const fileGone = !ab.pageContainsText('test-delete.txt');
  assertCondition(fileGone, 'T6.2: 文件从列表消失');
  ab.screenshot('t06-03-after-delete');

  // T6.3 - 进入回收站
  ab.findAndClick('回收站');
  ab.waitLoad('networkidle');
  ab.waitMs(1000);
  const inRecycle = ab.pageContainsText('test-delete.txt');
  assertCondition(inRecycle, 'T6.3: 回收站中可见被删除的文件');
  ab.screenshot('t06-04-recycle-bin');

  // T6.4 - 恢复文件
  if (inRecycle) {
    snap = ab.snapshot();
    const restoreBtn = ab.findRefByText(snap, '恢复');
    if (restoreBtn) {
      ab.click(restoreBtn);
      ab.waitLoad('networkidle');
      ab.waitMs(1000);
      const restored = !ab.pageContainsText('test-delete.txt');
      step('T6.4: 文件已恢复', restored);
      ab.screenshot('t06-05-restored');
    } else {
      step('T6.4: 找不到恢复按钮', false);
    }
  }

  // T6.5 - 返回文件列表验证恢复
  ab.findAndClick('全部文件');
  ab.waitLoad('networkidle');
  ab.waitMs(1000);
  const backInList = ab.pageContainsText('test-delete.txt');
  step('T6.5: 恢复的文件重新出现在列表中', backInList);
  ab.screenshot('t06-06-back-in-list');

  // T6.6 & T6.7 - 彻底删除
  snap = ab.snapshot();
  const actionBtn2 = ab.findRefByText(snap, '操作') || ab.findRefByRole(snap, 'button', '...');
  if (actionBtn2) {
    ab.click(actionBtn2);
    ab.waitMs(500);
    snap = ab.snapshot();
    const delItem = ab.findRefByText(snap, '删除');
    if (delItem) ab.click(delItem);
    ab.waitMs(500);
    snap = ab.snapshot();
    const confirmDel = ab.findRefByRole(snap, 'button', '确定');
    if (confirmDel) ab.click(confirmDel);
    ab.waitLoad('networkidle');
    ab.waitMs(1000);
  }
  step('T6.6: 文件删除并移至回收站', true);

  ab.findAndClick('回收站');
  ab.waitLoad('networkidle');
  ab.waitMs(1000);
  snap = ab.snapshot();
  const permDelBtn = ab.findRefByText(snap, '彻底删除');
  if (permDelBtn) {
    ab.click(permDelBtn);
    ab.waitMs(500);
    snap = ab.snapshot();
    const confirmPerm = ab.findRefByRole(snap, 'button', '确定') || ab.findRefByRole(snap, 'button', '彻底删除');
    if (confirmPerm) ab.click(confirmPerm);
    ab.waitLoad('networkidle');
    ab.waitMs(1000);
  }
  ab.screenshot('t06-07-permanent-deleted');

  try { fs.unlinkSync(tmpFile); } catch {}
  ab.closeBrowser();
});
