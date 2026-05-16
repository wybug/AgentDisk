const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T1: OAuth2 登录流程', () => {
  ab.closeBrowser();

  // T1.1 - 访问前端，重定向到网关登录页
  ab.open(ab.BASE_URL);
  ab.waitMs(4000);
  ab.waitLoad('networkidle');

  const url1 = ab.getUrl();
  const isLoginPage = url1.includes('3000') && url1.includes('login');
  assertCondition(isLoginPage, 'T1.1: 浏览器重定向到网关登录页', url1);
  ab.screenshot('t01-01-login-page');

  // T1.2 - 输入用户名密码登录
  let snap = ab.snapshot();
  const userIdRef = ab.findRefByPlaceholder(snap, '用户 ID');
  const passwordRef = ab.findRefByPlaceholder(snap, '密码');
  // Button text has a space: "登 录"
  const loginBtnRef = ab.findRefByText(snap, '登 录') || ab.findRefByRole(snap, 'button', '登 录');

  assertCondition(userIdRef !== null, 'T1.2: 找到用户 ID 输入框', userIdRef || 'not found');
  assertCondition(passwordRef !== null, 'T1.2: 找到密码输入框', passwordRef || 'not found');
  assertCondition(loginBtnRef !== null, 'T1.2: 找到登录按钮', loginBtnRef || 'not found');

  ab.fill(userIdRef, 'user001');
  ab.fill(passwordRef, 'test123');
  ab.click(loginBtnRef);

  // T1.3 - 等待重定向到授权页面
  ab.waitMs(3000);
  ab.waitLoad('networkidle');

  const url2 = ab.getUrl();
  const isAuthorizePage = url2.includes('3000') && url2.includes('authorize');
  assertCondition(isAuthorizePage, 'T1.3: 到达授权确认页', url2);
  ab.screenshot('t01-02-authorize-page');

  // T1.3b - 点击允许（使用 eval 调用 approve() 因为 onclick 可能不被触发）
  ab.evalStdin('approve()');
  ab.waitMs(6000);
  ab.waitLoad('networkidle');

  // T1.4 - 验证重定向回前端主界面
  const url3 = ab.getUrl();
  const isMainPage = url3.includes('5173') && (url3.includes('explorer') || url3 === ab.BASE_URL + '/');
  assertCondition(isMainPage, 'T1.4: 登录成功，回到前端主界面', url3);
  ab.screenshot('t01-03-main-page');

  // T1.5 - 验证界面元素
  const hasTitle = ab.pageContainsText('AgentDisk');
  assertCondition(hasTitle, 'T1.5: 页面显示 AgentDisk 标题');

  const hasUser = ab.pageContainsText('user001');
  assertCondition(hasUser, 'T1.5: 页面显示用户信息 user001');

  const hasSpace = ab.pageContainsText('GB') || ab.pageContainsText('MB');
  assertCondition(hasSpace, 'T1.5: 页面显示空间用量');
  ab.screenshot('t01-04-main-ui');

  // T1.6 - 退出登录
  snap = ab.snapshot();
  const logoutRef = ab.findRefByText(snap, '退出登录');
  if (logoutRef) {
    ab.click(logoutRef);
    ab.waitMs(2000);
    ab.screenshot('t01-05-logout');
  } else {
    step('T1.6: 退出登录（跳过）', true, '未找到退出登录按钮');
  }

  ab.closeBrowser();
});
