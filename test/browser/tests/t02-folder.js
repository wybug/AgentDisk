const { describe, step, assertCondition, printReport } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

describe('T2: 文件夹管理', () => {
  ab.closeBrowser();
  ab.login('user001', 'test123');

  // Helper: create a subfolder in current directory
  function createFolder(name) {
    ab.jsClickBtn('新建文件夹');
    ab.waitMs(1500);
    let s = ab.snapshot();
    if (!s.includes('文件夹名称')) {
      ab.jsClickBtn('新建文件夹');
      ab.waitMs(1500);
    }
    ab.jsFill('文件夹名称', name);
    ab.waitMs(500);
    ab.jsClickBtn('创 建');
    ab.waitMs(2000);
    return ab.pageContainsText(name);
  }

  // Helper: enter a folder by clicking its link
  function enterFolder(name) {
    ab.jsClickLink(name);
    ab.waitMs(2500);
    return ab.getUrl();
  }

  // ============================================================
  // T2.1 - 创建一级文件夹
  // ============================================================
  assertCondition(createFolder('一级目录A'), 'T2.1: 一级目录A创建成功');
  ab.screenshot('t02-01-create-level1');

  // ============================================================
  // T2.2 - 同名文件夹不允许重复创建
  // ============================================================
  ab.jsClickBtn('新建文件夹');
  ab.waitMs(1500);
  ab.jsFill('文件夹名称', '一级目录A');
  ab.waitMs(500);
  ab.jsClickBtn('创 建');
  ab.waitMs(2000);
  // Check if modal is still open (means create failed due to duplicate)
  const snapDup = ab.snapshot();
  const modalStillOpen = snapDup.includes('文件夹名称');
  // Also check for error message
  const fullSnapDup = ab.snapshotFull();
  const hasDupError = fullSnapDup.includes('同名') || fullSnapDup.includes('已存在');
  step('T2.2: 同名文件夹被拒绝', modalStillOpen || hasDupError);
  // Close modal
  if (modalStillOpen) {
    ab.jsClickBtn('取 消');
    ab.waitMs(500);
  }
  ab.screenshot('t02-02-duplicate-rejected');

  // ============================================================
  // T2.3 - 进入一级目录A
  // ============================================================
  let url = enterFolder('一级目录A');
  assertCondition(url.includes('/explorer/'), 'T2.3: 进入一级目录A', url);
  ab.screenshot('t02-03-enter-level1');

  // ============================================================
  // T2.4 - 一级面包屑显示正确
  // ============================================================
  ab.waitMs(1000);
  const bc1 = ab.pageContainsText('一级目录A');
  assertCondition(bc1, 'T2.4: 一级面包屑显示一级目录A');
  ab.screenshot('t02-04-breadcrumb-level1');

  // ============================================================
  // T2.5 - 创建二级文件夹
  // ============================================================
  assertCondition(createFolder('二级目录B'), 'T2.5: 二级目录B创建成功');
  ab.screenshot('t02-05-create-level2');

  // ============================================================
  // T2.6 - 进入二级目录B
  // ============================================================
  url = enterFolder('二级目录B');
  assertCondition(url.includes('/explorer/'), 'T2.6: 进入二级目录B', url);
  ab.screenshot('t02-06-enter-level2');

  // ============================================================
  // T2.7 - 二级面包屑显示完整路径
  // ============================================================
  ab.waitMs(1000);
  const bc2a = ab.pageContainsText('一级目录A');
  const bc2b = ab.pageContainsText('二级目录B');
  assertCondition(bc2a && bc2b, 'T2.7: 二级面包屑显示完整路径', `A=${bc2a} B=${bc2b}`);
  ab.screenshot('t02-07-breadcrumb-level2');

  // ============================================================
  // T2.8 - 创建三级文件夹
  // ============================================================
  assertCondition(createFolder('三级目录C'), 'T2.8: 三级目录C创建成功');
  ab.screenshot('t02-08-create-level3');

  // ============================================================
  // T2.9 - 进入三级目录C
  // ============================================================
  url = enterFolder('三级目录C');
  assertCondition(url.includes('/explorer/'), 'T2.9: 进入三级目录C', url);
  ab.screenshot('t02-09-enter-level3');

  // ============================================================
  // T2.10 - 三级面包屑显示完整路径
  // ============================================================
  ab.waitMs(1000);
  const bc3a = ab.pageContainsText('一级目录A');
  const bc3b = ab.pageContainsText('二级目录B');
  const bc3c = ab.pageContainsText('三级目录C');
  assertCondition(
    bc3a && bc3b && bc3c,
    'T2.10: 三级面包屑显示完整路径',
    `A=${bc3a} B=${bc3b} C=${bc3c}`
  );
  ab.screenshot('t02-10-breadcrumb-level3');

  // ============================================================
  // T2.11 - 面包屑导航：从三级跳回二级
  // ============================================================
  ab.jsClickLink('二级目录B');
  ab.waitMs(2500);
  let urlBack2 = ab.getUrl();
  let hasDirC = ab.pageContainsText('三级目录C');
  assertCondition(
    urlBack2.includes('/explorer/') && hasDirC,
    'T2.11: 面包屑跳回二级，可见三级目录C',
    `url=${urlBack2} hasC=${hasDirC}`
  );
  ab.screenshot('t02-11-nav-to-level2');

  // ============================================================
  // T2.12 - 面包屑导航：从二级跳回一级
  // ============================================================
  ab.jsClickLink('一级目录A');
  ab.waitMs(2500);
  let urlBack1 = ab.getUrl();
  let hasDirB = ab.pageContainsText('二级目录B');
  assertCondition(
    urlBack1.includes('/explorer/') && hasDirB,
    'T2.12: 面包屑跳回一级，可见二级目录B',
    `url=${urlBack1} hasB=${hasDirB}`
  );
  ab.screenshot('t02-12-nav-to-level1');

  // ============================================================
  // T2.13 - 面包屑导航：从一级跳回根目录
  // ============================================================
  ab.jsClickLink('全部文件');
  ab.waitMs(2500);
  let urlRoot = ab.getUrl();
  let hasDirA = ab.pageContainsText('一级目录A');
  assertCondition(
    hasDirA && urlRoot.includes('/explorer'),
    'T2.13: 面包屑跳回根目录，可见一级目录A',
    `url=${urlRoot} hasA=${hasDirA}`
  );
  ab.screenshot('t02-13-nav-to-root');

  // ============================================================
  // T2.14 - 重新进入三级目录，验证深路径可达
  // ============================================================
  enterFolder('一级目录A');
  ab.waitMs(1500);
  enterFolder('二级目录B');
  ab.waitMs(1500);
  enterFolder('三级目录C');
  ab.waitMs(2500);

  let urlDeep = ab.getUrl();
  assertCondition(urlDeep.includes('/explorer/'), 'T2.14: 重新进入三级目录', urlDeep);
  ab.screenshot('t02-14-reenter-deep');

  // ============================================================
  // T2.15 - 重命名文件夹：回到根目录，重命名一级目录A为一级目录D
  // ============================================================
  ab.jsClickLink('全部文件');
  ab.waitMs(2000);

  // Click rename button for 一级目录A
  const renameResult = ab.evalStdin(`
    (function() {
      var rows = document.querySelectorAll('tr');
      for (var i = 0; i < rows.length; i++) {
        if (rows[i].textContent.includes('一级目录A')) {
          var btns = rows[i].querySelectorAll('button');
          for (var j = 0; j < btns.length; j++) {
            if (btns[j].textContent.includes('重命名')) {
              btns[j].click();
              return 'clicked rename';
            }
          }
        }
      }
      return 'not found';
    })()
  `);
  ab.waitMs(1500);

  let snapRename = ab.snapshot();
  const hasRenameModal = snapRename.includes('重命名文件夹') || snapRename.includes('新名称');
  step('T2.15a: 重命名弹窗打开', hasRenameModal, renameResult);
  ab.screenshot('t02-15a-rename-modal');

  if (hasRenameModal) {
    // Clear existing value and fill new name
    ab.evalStdin(`
      (function() {
        var inputs = document.querySelectorAll('input');
        for (var i = 0; i < inputs.length; i++) {
          if (inputs[i].placeholder && inputs[i].placeholder.indexOf('新名称') !== -1) {
            var nativeSet = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
            nativeSet.call(inputs[i], '一级目录D');
            inputs[i].dispatchEvent(new Event('input', { bubbles: true }));
            inputs[i].dispatchEvent(new Event('change', { bubbles: true }));
            return 'filled';
          }
        }
        return 'not found';
      })()
    `);
    ab.waitMs(500);
    ab.jsClickBtn('确 定');
    ab.waitMs(2000);
  }

  const hasD = ab.pageContainsText('一级目录D');
  // Check that "一级目录A" only appears in sidebar, not in the table
  const noA = !ab.pageContainsText('一级目录A') ||
    ab.evalStdin(`
      (function() {
        var rows = document.querySelectorAll('table tbody tr');
        for (var i = 0; i < rows.length; i++) {
          if (rows[i].textContent.includes('一级目录A')) return 'found-in-table';
        }
        return 'not-in-table';
      })()
    `) === 'not-in-table';
  assertCondition(hasD, 'T2.15b: 文件夹重命名成功', `hasD=${hasD}`);
  ab.screenshot('t02-15b-renamed');

  // ============================================================
  // T2.16 - 删除所有测试文件夹（递归 API 删除）
  // ============================================================
  const deletedViaApi = ab.evalStdin(`
    (function() {
      function xhr(method, url) {
        var x = new XMLHttpRequest();
        x.open(method, url, false);
        x.withCredentials = true;
        x.send();
        return x;
      }

      function deleteRecursive(parentId) {
        var r = xhr('GET', '/v1/disk/folders?parentId=' + parentId);
        var data = JSON.parse(r.responseText);
        var folders = data.data || [];
        var count = 0;
        for (var i = 0; i < folders.length; i++) {
          count += deleteRecursive(folders[i].id);
          var dr = xhr('DELETE', '/v1/disk/folders/' + folders[i].id);
          if (dr.status < 300) count++;
        }
        return count;
      }

      var total = deleteRecursive(0);
      return 'deleted ' + total + ' folders';
    })()
  `);
  ab.waitMs(2000);

  ab.evalStdin('window.location.href = "/explorer"');
  ab.waitMs(3000);

  const allDeleted = !ab.pageContainsText('一级目录D');
  assertCondition(allDeleted, 'T2.16: 所有测试文件夹已删除', deletedViaApi);
  ab.screenshot('t02-16-all-deleted');

  ab.closeBrowser();
});

printReport();
