const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

function goBackToExplorer() {
  ab.evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        var t = btns[i].textContent;
        if (t.includes('返回') || t.includes('文件列表')) { btns[i].click(); return 'clicked: ' + t; }
      }
      window.location.href = '/explorer';
      return 'navigated';
    })()
  `);
  ab.waitLoad('networkidle');
  ab.waitMs(1500);
}

// Click file link by name using JS — match first link (newest, since list is DESC)
function clickFileLink(fileName) {
  return ab.evalStdin(`
    (function() {
      var links = document.querySelectorAll('.ant-table-row a');
      for (var i = 0; i < links.length; i++) {
        if (links[i].textContent.includes('${fileName}')) { links[i].click(); return 'clicked link'; }
      }
      return 'not found';
    })()
  `);
}

describe('T4: 文件预览', () => {
  ab.closeAll();
  ab.login('user001', 'test123');

  // 确保在 explorer 页面
  ab.waitMs(2000);
  const currentUrl = ab.getUrl();
  if (!currentUrl.includes('/explorer')) {
    ab.jsClickElement('.ant-menu-item');
    ab.waitMs(2000);
  }

  // T4.1 - 点击 md 文件名称预览
  let snap = ab.snapshot();
  const mdFile = ab.findRefByText(snap, 'agentdisk-test-upload.md');
  if (mdFile) {
    ab.click(mdFile);
    ab.waitLoad('networkidle');
    ab.waitMs(2000);
    ab.screenshot('t04-01-md-preview');

    const hasMdContent = ab.pageContainsText('Markdown') || ab.pageContainsText('item');
    assertCondition(hasMdContent, 'T4.1: Markdown 预览正常渲染');

    // T4.2 - 返回文件列表
    goBackToExplorer();
    const backUrl = ab.getUrl();
    assertCondition(backUrl.includes('/explorer'), 'T4.2: 返回文件列表', backUrl);
    ab.screenshot('t04-02-back-to-list');
  } else {
    step('T4.1: 找不到 md 文件，跳过预览测试', false, '请先运行 T3 上传文件');
    ab.screenshot('t04-01-no-md-file');
    ab.closeBrowser();
    return;
  }

  // T4.3 - 点击 py 代码文件预览（用 JS 精确点击 link 元素）
  const pyClicked = clickFileLink('agentdisk-test-upload.py');
  ab.waitMs(3000);
  ab.screenshot('t04-03-py-preview');

  const pageText = ab.evalStdin('document.body.innerText');
  const hasCodeContent = pageText.includes('def') || pageText.includes('print') || pageText.includes('hello');
  assertCondition(hasCodeContent, 'T4.3: 代码文件预览正常', hasCodeContent ? '代码内容可见' : 'click=' + pyClicked + ' content=' + pageText.substring(0, 150));

  goBackToExplorer();

  // T4.5 - 点击 txt 文件预览
  const txtClicked = clickFileLink('agentdisk-test-upload.txt');
  ab.waitMs(3000);
  ab.screenshot('t04-05-txt-preview');

  const pageText2 = ab.evalStdin('document.body.innerText');
  const hasTxtContent = pageText2.includes('Hello') || pageText2.includes('AgentDisk');
  assertCondition(hasTxtContent, 'T4.5: 纯文本预览正常', hasTxtContent ? '文本内容可见' : 'click=' + txtClicked + ' content=' + pageText2.substring(0, 150));

  goBackToExplorer();

  ab.closeBrowser();
});
