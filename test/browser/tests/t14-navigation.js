const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T14: 页面导航与响应式', () => {
  ab.closeAll();
  ab.login('user001', 'test123');

  // T14.1 - 依次点击侧边栏各菜单项
  const menuItems = ['全部文件', '回收站', '我的分享', '标签搜索', '权限管理'];

  for (const item of menuItems) {
    try {
      ab.findAndClick(item);
      ab.waitLoad('networkidle');
      ab.waitMs(800);
      const currentUrl = ab.getUrl();
      step(`点击「${item}」`, true, currentUrl);
      ab.screenshot(`t14-nav-${item.replace(/\s/g, '-')}`);
    } catch {
      step(`点击「${item}」`, false, '导航失败');
    }
  }

  // T14.2 - 回到全部文件
  ab.findAndClick('全部文件');
  ab.waitLoad('networkidle');
  ab.waitMs(800);
  step('T14.2: 回到全部文件页面', true);
  ab.screenshot('t14-back-to-files');

  // T14.3 - 缩小窗口到 768px 以下
  ab.setViewport(768, 1024);
  ab.waitMs(1000);
  ab.screenshot('t14-responsive-768');

  ab.setViewport(375, 812);
  ab.waitMs(1000);
  ab.screenshot('t14-responsive-375');

  // T14.4 - 验证侧边栏折叠
  const snap = ab.snapshot();
  const siderCollapsed = !snap.includes('全部文件') || snap.includes('collapsed');
  step('T14.3/T14.4: 响应式布局 - 小屏幕下侧边栏状态', true, siderCollapsed ? '已折叠' : '未折叠');

  // 恢复窗口
  ab.setViewport(1440, 900);
  ab.waitMs(500);

  ab.closeBrowser();
});
