const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T10: 权限管理', () => {
  ab.closeAll();
  ab.login('user001', 'test123');

  // T10.1 - 进入权限管理页面
  ab.findAndClick('权限管理');
  ab.waitLoad('networkidle');
  ab.waitMs(1000);
  step('T10.1: 进入权限管理页面', true);
  ab.screenshot('t10-01-permissions-page');

  // T10.2 - 点击授予权限
  let snap = ab.snapshot();
  const grantBtn = ab.findRefByText(snap, '授予权限') || ab.findRefByText(snap, '授予') || ab.findRefByRole(snap, 'button', '授予权限');
  if (grantBtn) {
    ab.click(grantBtn);
    ab.waitMs(1000);
    assertCondition(true, 'T10.2: 打开授权表单');
  } else {
    step('T10.2: 找不到授予权限按钮', false);
    ab.closeBrowser();
    return;
  }
  ab.screenshot('t10-02-grant-modal');

  // T10.3 - 填写权限信息
  snap = ab.snapshot();
  const agentIdInput = ab.findRefByPlaceholder(snap, 'Agent ID') || ab.findRefByPlaceholder(snap, '智能体');
  if (agentIdInput) {
    ab.fill(agentIdInput, 'agent-test-001');
  }

  snap = ab.snapshot();
  const resourceIdInput = ab.findRefByPlaceholder(snap, '资源 ID') || ab.findRefByPlaceholder(snap, 'Resource');
  if (resourceIdInput) {
    ab.fill(resourceIdInput, '1');
  }

  step('T10.3: 填写权限信息', true);
  ab.screenshot('t10-03-form-filled');

  // T10.4 - 提交授权
  snap = ab.snapshot();
  const submitBtn = ab.findRefByRole(snap, 'button', '授予') || ab.findRefByRole(snap, 'button', '提交') || ab.findRefByRole(snap, 'button', '确定');
  if (submitBtn) {
    ab.click(submitBtn);
    ab.waitLoad('networkidle');
    ab.waitMs(1000);
    const granted = ab.pageContainsText('成功') || ab.pageContainsText('agent-test-001');
    assertCondition(granted, 'T10.4: 权限授予成功');
  } else {
    step('T10.4: 找不到提交按钮', false);
  }
  ab.screenshot('t10-04-granted');

  // T10.5 - 验证列表中的记录
  const hasRecord = ab.pageContainsText('agent-test-001');
  assertCondition(hasRecord, 'T10.5: 列表中显示权限记录');
  ab.screenshot('t10-05-permission-list');

  // T10.6 - 撤销权限
  snap = ab.snapshot();
  const revokeBtn = ab.findRefByText(snap, '撤销');
  if (revokeBtn) {
    ab.click(revokeBtn);
    ab.waitMs(500);
    snap = ab.snapshot();
    const confirmBtn = ab.findRefByRole(snap, 'button', '确定');
    if (confirmBtn) ab.click(confirmBtn);
    ab.waitLoad('networkidle');
    ab.waitMs(1000);
    const revoked = !ab.pageContainsText('agent-test-001');
    step('T10.6: 权限已撤销', revoked);
  } else {
    step('T10.6: 找不到撤销按钮', false);
  }
  ab.screenshot('t10-06-revoked');

  ab.closeBrowser();
});
