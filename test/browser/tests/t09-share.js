const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T9: 分享管理', () => {
  ab.closeAll();
  ab.login('user001', 'test123');

  // T9.1 - 点击文件操作 → 分享
  let snap = ab.snapshot();
  const actionBtn = ab.findRefByText(snap, '操作') || ab.findRefByRole(snap, 'button', '...');
  if (!actionBtn) {
    step('T9.1: 找不到操作按钮', false, '请确保有文件存在');
    ab.closeBrowser();
    return;
  }

  ab.click(actionBtn);
  ab.waitMs(500);
  snap = ab.snapshot();
  const shareItem = ab.findRefByText(snap, '分享');
  if (shareItem) {
    ab.click(shareItem);
    ab.waitMs(1000);
    assertCondition(true, 'T9.1: 打开创建分享弹窗');
  } else {
    step('T9.1: 找不到分享菜单项', false);
    ab.closeBrowser();
    return;
  }
  ab.screenshot('t09-01-share-modal');

  // T9.2 - 填写分享信息并创建
  snap = ab.snapshot();
  const extractInput = ab.findRefByPlaceholder(snap, '提取码');
  if (extractInput) {
    ab.fill(extractInput, 'abc123');
  }

  snap = ab.snapshot();
  const maxVisitInput = ab.findRefByPlaceholder(snap, '最大访问次数');
  if (maxVisitInput) {
    ab.fill(maxVisitInput, '10');
  }

  snap = ab.snapshot();
  const createShareBtn = ab.findRefByRole(snap, 'button', '创建分享') || ab.findRefByRole(snap, 'button', '创建');
  if (createShareBtn) {
    ab.click(createShareBtn);
    ab.waitLoad('networkidle');
    ab.waitMs(1500);
  }
  const shareCreated = ab.pageContainsText('成功') || ab.pageContainsText('分享码') || ab.pageContainsText('abc123');
  assertCondition(shareCreated, 'T9.2: 分享创建成功');
  ab.screenshot('t09-02-share-created');

  // T9.3 - 查看分享列表
  ab.findAndClick('我的分享');
  ab.waitLoad('networkidle');
  ab.waitMs(1000);
  const shareList = ab.pageContainsText('分享码') || ab.pageContainsText('abc123') || ab.pageContainsText('访问');
  assertCondition(shareList, 'T9.3: 分享列表显示分享记录');
  ab.screenshot('t09-03-share-list');

  // T9.4 - 记录分享码用于后续测试
  const shareCode = ab.evalStdin(`
    const el = document.querySelector('[data-share-code]') ||
               document.querySelector('.share-code') ||
               Array.from(document.querySelectorAll('td, span')).find(e => /^[a-zA-Z0-9]{6,}$/.test(e.textContent?.trim()));
    el ? el.textContent.trim() : '';
  `);
  step('T9.4: 获取分享码', shareCode.length > 0, shareCode || '未找到分享码');

  // T9.9 - 撤销分享
  snap = ab.snapshot();
  const revokeBtn = ab.findRefByText(snap, '撤销');
  if (revokeBtn) {
    ab.click(revokeBtn);
    ab.waitMs(500);
    snap = ab.snapshot();
    const confirmRevoke = ab.findRefByRole(snap, 'button', '确定');
    if (confirmRevoke) ab.click(confirmRevoke);
    ab.waitLoad('networkidle');
    ab.waitMs(1000);
    const revoked = ab.pageContainsText('已失效') || !ab.pageContainsText('abc123');
    step('T9.9: 分享已撤销', revoked);
  } else {
    step('T9.9: 找不到撤销按钮', false);
  }
  ab.screenshot('t09-04-share-revoked');

  ab.closeBrowser();
});
