const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

function navigateTo(page) {
  return ab.evalStdin(`
    (function() {
      var items = document.querySelectorAll('.ant-menu-item');
      for (var i = 0; i < items.length; i++) {
        if (items[i].textContent.includes('${page}')) { items[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
}

describe('T16: 页面导航与响应式', () => {
  ab.closeAll();
  ab.login('user001', 'test123');
  ab.waitMs(2000);

  // T16.1 - 依次点击侧边栏各菜单项，验证 URL
  const pages = [
    { menu: '回收站', url: '/recycle' },
    { menu: '我的分享', url: '/shares' },
    { menu: '标签搜索', url: '/tags' },
    { menu: '权限管理', url: '/permissions' },
  ];

  for (const p of pages) {
    navigateTo(p.menu);
    ab.waitMs(2000);
    const currentUrl = ab.getUrl();
    const ok = currentUrl.includes(p.url);
    assertCondition(ok, 'T16.1: 导航到「' + p.menu + '」', currentUrl);
    ab.screenshot('t14-nav-' + p.menu);
  }

  // T16.2 - 回到全部文件
  navigateTo('全部文件');
  ab.waitMs(2000);
  const urlFiles = ab.getUrl();
  assertCondition(urlFiles.includes('/explorer'), 'T16.2: 返回全部文件', urlFiles);
  ab.screenshot('t14-02-back-to-files');

  // T16.3 - 缩小窗口测试响应式
  ab.setViewport(768, 1024);
  ab.waitMs(1000);
  ab.screenshot('t14-03-responsive-768');

  ab.setViewport(375, 812);
  ab.waitMs(1000);
  ab.screenshot('t14-04-responsive-375');

  step('T16.3: 响应式视口测试完成', true, '768x1024 + 375x812');

  // 恢复窗口
  ab.setViewport(1440, 900);
  ab.waitMs(500);

  ab.closeBrowser();
});
