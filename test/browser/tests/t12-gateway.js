const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T12: 测试网关管理', () => {
  ab.closeAll();

  // T12.1 - 访问网关仪表盘
  ab.open(ab.GATEWAY_URL + '/dashboard');
  ab.waitLoad('networkidle');
  ab.waitMs(1500);
  const dashLoaded = ab.pageContainsText('AgentDisk') || ab.pageContainsText('网关') || ab.pageContainsText('OAuth2');
  assertCondition(dashLoaded, 'T12.1: 网关仪表盘加载成功');
  ab.screenshot('t12-01-dashboard');

  // T12.2 - 查看 OAuth2 配置区域
  const hasOAuth = ab.pageContainsText('Client ID') || ab.pageContainsText('OAuth2') || ab.pageContainsText('oauth2');
  assertCondition(hasOAuth, 'T12.2: OAuth2 配置信息显示');
  ab.screenshot('t12-02-oauth-config');

  // T12.3 - 添加测试用户
  let snap = ab.snapshot();
  const newUserIdInput = ab.findRefByPlaceholder(snap, '用户 ID') || ab.findRefByPlaceholder(snap, 'userId');
  if (newUserIdInput) {
    ab.fill(newUserIdInput, 'testuser999');

    snap = ab.snapshot();
    const newNameInput = ab.findRefByPlaceholder(snap, '用户名') || ab.findRefByPlaceholder(snap, 'userName');
    if (newNameInput) ab.fill(newNameInput, '测试用户');

    snap = ab.snapshot();
    const newPwdInput = ab.findRefByPlaceholder(snap, '密码') || ab.findRefByPlaceholder(snap, 'password');
    if (newPwdInput) ab.fill(newPwdInput, 'test123');

    snap = ab.snapshot();
    const addBtn = ab.findRefByRole(snap, 'button', '添加') || ab.findRefByText(snap, '添加');
    if (addBtn) {
      ab.click(addBtn);
      ab.waitLoad('networkidle');
      ab.waitMs(1000);
      const added = ab.pageContainsText('testuser999');
      step('T12.3: 测试用户添加成功', added);
    }
  } else {
    step('T12.3: 找不到用户添加表单', false);
  }
  ab.screenshot('t12-03-user-added');

  // T12.4 - 删除测试用户
  snap = ab.snapshot();
  const deleteBtn = ab.findRefByText(snap, '删除');
  if (deleteBtn) {
    ab.click(deleteBtn);
    ab.waitMs(500);
    snap = ab.snapshot();
    const confirmDel = ab.findRefByRole(snap, 'button', '确定');
    if (confirmDel) ab.click(confirmDel);
    ab.waitLoad('networkidle');
    ab.waitMs(500);
    const gone = !ab.pageContainsText('testuser999');
    step('T12.4: 测试用户删除成功', gone);
  } else {
    step('T12.4: 找不到删除按钮', false);
  }
  ab.screenshot('t12-04-user-deleted');

  // T12.5 - 打开 AgentDisk 按钮
  snap = ab.snapshot();
  const openBtn = ab.findRefByText(snap, '打开 AgentDisk') || ab.findRefByRole(snap, 'link', 'AgentDisk');
  step('T12.5: 打开 AgentDisk 按钮存在', openBtn !== null);

  ab.closeBrowser();
});
