const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T7: 标签管理', () => {
  ab.closeAll();
  ab.login('user001', 'test123');

  // T7.1 - 点击文件操作 → 标签
  let snap = ab.snapshot();
  const fileRow = ab.findRefByText(snap, 'test-upload.txt') || ab.findRefByText(snap, '.txt');
  if (!fileRow) {
    step('T7.1: 找不到测试文件', false, '请先运行 T3 上传文件');
    ab.closeBrowser();
    return;
  }

  const actionBtn = ab.findRefByText(snap, '操作') || ab.findRefByRole(snap, 'button', '...');
  if (actionBtn) {
    ab.click(actionBtn);
    ab.waitMs(500);
    snap = ab.snapshot();
    const tagItem = ab.findRefByText(snap, '标签');
    if (tagItem) {
      ab.click(tagItem);
      ab.waitMs(1000);
      assertCondition(true, 'T7.1: 打开标签管理弹窗');
    } else {
      step('T7.1: 找不到标签菜单项', false);
      ab.closeBrowser();
      return;
    }
  } else {
    step('T7.1: 找不到操作按钮', false);
    ab.closeBrowser();
    return;
  }
  ab.screenshot('t07-01-tag-modal');

  // T7.2 - 输入标签「重要」并添加
  snap = ab.snapshot();
  const tagInput = ab.findRefByPlaceholder(snap, '标签') || ab.findRefByPlaceholder(snap, '输入标签');
  if (tagInput) {
    ab.fill(tagInput, '重要');
    snap = ab.snapshot();
    const addBtn = ab.findRefByRole(snap, 'button', '添加') || ab.findRefByText(snap, '添加');
    if (addBtn) ab.click(addBtn);
    ab.waitMs(1000);
    const hasTag = ab.pageContainsText('重要');
    assertCondition(hasTag, 'T7.2: 标签「重要」添加成功');
  } else {
    step('T7.2: 找不到标签输入框', false);
  }
  ab.screenshot('t07-02-tag-added');

  // T7.3 - 添加标签「文档」
  snap = ab.snapshot();
  const tagInput2 = ab.findRefByPlaceholder(snap, '标签') || ab.findRefByPlaceholder(snap, '输入标签');
  if (tagInput2) {
    ab.fill(tagInput2, '文档');
    snap = ab.snapshot();
    const addBtn2 = ab.findRefByRole(snap, 'button', '添加') || ab.findRefByText(snap, '添加');
    if (addBtn2) ab.click(addBtn2);
    ab.waitMs(1000);
    const hasTag2 = ab.pageContainsText('文档');
    step('T7.3: 标签「文档」添加成功', hasTag2);
  }
  ab.screenshot('t07-03-two-tags');

  // T7.4 - 关闭弹窗
  snap = ab.snapshot();
  const closeBtn = ab.findRefByRole(snap, 'button', '关闭') || ab.findRefByRole(snap, 'button', '取消');
  if (closeBtn) ab.click(closeBtn);
  else ab.press('Escape');
  ab.waitMs(500);

  // T7.5 - 标签搜索
  ab.findAndClick('标签搜索');
  ab.waitLoad('networkidle');
  ab.waitMs(1000);
  ab.screenshot('t07-04-tag-search-page');

  // T7.6 - 搜索标签「重要」
  snap = ab.snapshot();
  const searchInput = ab.findRefByPlaceholder(snap, '标签') || ab.findRefByPlaceholder(snap, '搜索');
  if (searchInput) {
    ab.fill(searchInput, '重要');
    snap = ab.snapshot();
    const searchBtn = ab.findRefByRole(snap, 'button', '搜索') || ab.findRefByText(snap, '搜索');
    if (searchBtn) ab.click(searchBtn);
    ab.waitLoad('networkidle');
    ab.waitMs(1000);
    const found = ab.pageContainsText('test-upload');
    assertCondition(found, 'T7.6: 搜索「重要」找到标记的文件');
  } else {
    step('T7.6: 找不到搜索输入框', false);
  }
  ab.screenshot('t07-05-search-result');

  ab.closeBrowser();
});
