const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T2: 文件夹管理', () => {
  ab.closeBrowser();
  ab.login('user001', 'test123');

  // T2.1 - 点击新建文件夹，验证弹窗打开
  ab.jsClickBtn('新建文件夹');
  ab.waitMs(1500);
  let snap = ab.snapshot();
  const hasModal = snap.includes('文件夹名称');
  assertCondition(hasModal, 'T2.1: 新建文件夹弹窗打开');
  ab.screenshot('t02-01-create-modal');

  // T2.2 - 输入文件夹名称并创建
  ab.jsFill('文件夹名称', '测试文件夹A');
  ab.waitMs(500);
  ab.jsClickBtn('创 建');
  ab.waitMs(2000);

  const hasFolder = ab.pageContainsText('测试文件夹A');
  assertCondition(hasFolder, 'T2.2: 文件夹「测试文件夹A」创建成功');
  ab.screenshot('t02-02-folder-created');

  // T2.3 - 点击进入文件夹
  ab.jsClickLink('测试文件夹A');
  ab.waitMs(2000);
  const url3 = ab.getUrl();
  assertCondition(url3.includes('/explorer/'), 'T2.3: 进入子文件夹', url3);
  ab.screenshot('t02-03-entered-folder');

  // T2.4 - 在子文件夹中创建「子文件夹B」
  ab.waitMs(1000);
  snap = ab.snapshot();
  step('T2.4: 子文件夹页面已加载', snap.includes('暂无数据') || snap.includes('文件夹'));

  // Click new folder button - may need to wait for page to be fully rendered
  ab.jsClickBtn('新建文件夹');
  ab.waitMs(2000);

  // Verify modal appeared
  snap = ab.snapshot();
  const hasSubModal = snap.includes('文件夹名称');
  if (!hasSubModal) {
    // Retry
    ab.jsClickBtn('新建文件夹');
    ab.waitMs(2000);
    snap = ab.snapshot();
  }

  ab.jsFill('文件夹名称', '子文件夹B');
  ab.waitMs(500);
  ab.jsClickBtn('创 建');
  ab.waitMs(2000);
  const hasSub = ab.pageContainsText('子文件夹B');
  assertCondition(hasSub, 'T2.4: 子文件夹创建成功');
  ab.screenshot('t02-04-subfolder-created');

  // T2.5 - 面包屑返回根目录
  ab.jsClickLink('全部文件');
  ab.waitMs(1500);
  const rootVisible = ab.pageContainsText('测试文件夹A');
  assertCondition(rootVisible, 'T2.5: 返回根目录，可见文件夹');
  ab.screenshot('t02-05-back-to-root');

  // T2.6 & T2.7 - 删除文件夹
  snap = ab.snapshot();
  const deleteBtn = ab.findRefByText(snap, '删 除') || ab.findRefByText(snap, '删除');
  if (deleteBtn) {
    ab.click(deleteBtn);
    ab.waitMs(1000);
    ab.screenshot('t02-06-delete-confirm');
    ab.jsClickBtn('确 定');
    ab.waitMs(2000);
    const deleted = !ab.pageContainsText('测试文件夹A');
    assertCondition(deleted, 'T2.7: 文件夹删除成功');
  } else {
    step('T2.6: 找到删除按钮', false, 'not found');
  }
  ab.screenshot('t02-07-after-delete');

  ab.closeBrowser();
});
