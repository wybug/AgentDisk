/**
 * T24: 协议层端到端 — 上传 → 分享 → 下载 → 预览 → 撤销 跨层验证
 *
 * 与 T06 (UI 预览) 和 T11 (UI 分享) 的区别：
 *   - T06 验证前端 UI 渲染（点击文件 → 看到 iframe/代码）
 *   - T11 验证分享管理 UX（列表/撤销/提取码交互）
 *   - T24 跳过 UI，直接用 fetch 打协议层：上传 markdown → 创建分享 →
 *     验证 4 个端点的 wire-format 契约（公开 GET、access、download、
 *     preview JSON、preview HTML 原文）→ 撤销 → 验证失效
 *
 * 这种"跨层"测试是为了捕获 SDK/前端/后端任何一侧悄悄改了 JSON 字段
 * 名或路径，但单元测试都通过的情况。
 *
 * 用法: node runner.js t24
 */
const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');

const TEST_FILENAME = 't24-e2e-protocol.md';
const TEST_CONTENT = '# T24 E2E\n\nprotocol cross-layer test body';

function uploadMarkdown() {
  return ab.evalStdin(`
    (function() {
      var blob = new Blob([${JSON.stringify(TEST_CONTENT)}], { type: 'text/markdown' });
      var fd = new FormData();
      fd.append('file', blob, '${TEST_FILENAME}');
      return fetch('/v1/disk/files/upload?folderId=0', {
        method: 'POST', body: fd, credentials: 'include'
      }).then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: id=' + d.data.id + ' name=' + d.data.fileName;
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function createShare(fileId, extractCode) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/shares', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          resourceId: ${fileId},
          resType: 'file',
          extractCode: '${extractCode}',
          maxVisit: 10,
          expireHours: 1
        }),
        credentials: 'include'
      }).then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== 0) return 'ERROR: ' + d.message;
          return 'OK: id=' + d.data.id + ' code=' + d.data.shareCode;
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function deleteFile(fileId) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/files/${fileId}', {
        method: 'DELETE', credentials: 'include'
      }).then(function(r) { return r.json(); })
        .then(function(d) { return d.code === 0 ? 'OK' : 'ERROR: ' + d.message; })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

// Cross-layer fetches done in a single browser eval to avoid round-trip
// overhead. Returns an array of {step, ok, detail} objects; evalStdin
// JSON-stringifies the result, so we JSON.parse it on this side to recover
// the structured data. The share download token + actual file fetch must
// happen in the same browser context (the downloadUrl is a minio presigned
// URL that the browser can read directly).
function exerciseProtocolStack(fileId, shareCode, extractCode) {
  const raw = ab.evalStdin(`
    (function() {
      var log = [];
      function rec(step, ok, detail) { log.push({ step: step, ok: ok, detail: detail }); }
      return Promise.resolve()
        .then(function() {
          return fetch('/v1/disk/share/' + '${shareCode}');
        })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          rec('publicGET', d.code === 0 && d.data && d.data.isActive === true, 'code=' + d.code + ' active=' + (d.data && d.data.isActive));
        })
        .then(function() {
          return fetch('/v1/disk/share/access', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ code: '${shareCode}', extractCode: '${extractCode}' })
          });
        })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          rec('access', d.code === 0, 'code=' + d.code + ' resourceId=' + (d.data && d.data.resourceId));
        })
        .then(function() {
          return fetch('/v1/disk/share/download', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ code: '${shareCode}', resourceId: ${fileId}, extractCode: '${extractCode}' })
          });
        })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.code !== 0) { rec('download', false, 'error ' + d.message); throw new Error('stop'); }
          var token = d.data.downloadToken;
          return fetch('/v1/disk/files/download?t=' + encodeURIComponent(token), {
            headers: { 'Accept': 'application/json' }
          })
            .then(function(r2) { return r2.json(); })
            .then(function(d2) {
              rec('downloadJSON', !!d2.data.downloadUrl, 'hasUrl=' + !!d2.data.downloadUrl);
              return fetch(d2.data.downloadUrl);
            })
            .then(function(r3) { return r3.text(); })
            .then(function(body) {
              rec('bytes', body === ${JSON.stringify(TEST_CONTENT)}, 'len=' + body.length + ' matches=' + (body === ${JSON.stringify(TEST_CONTENT)}));
            });
        })
        .then(function() {
          return fetch('/v1/disk/preview/' + ${fileId}, { credentials: 'include' });
        })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          rec('previewJSON', d.data && d.data.fileType === 'markdown', 'fileType=' + (d.data && d.data.fileType) + ' contentLen=' + (d.data && d.data.content || '').length);
        })
        .then(function() {
          return fetch('/v1/disk/preview/' + ${fileId} + '/html', { credentials: 'include' });
        })
        .then(function(r) {
          var csp = r.headers.get('content-security-policy') || '';
          var xcto = r.headers.get('x-content-type-options') || '';
          return r.text().then(function(body) {
            rec('previewHTML', r.status === 200 && !!csp && !!xcto, 'status=' + r.status + ' csp=' + (csp ? 'set' : 'MISSING') + ' xcto=' + (xcto ? 'set' : 'MISSING') + ' bodyLen=' + body.length);
          });
        })
        .then(function() { return log; })
        .catch(function(e) { log.push({ step: 'ABORT', ok: false, detail: e.message }); return log; });
    })()
  `);
  try {
    return JSON.parse(raw);
  } catch (e) {
    return [{ step: 'PARSE_FAIL', ok: false, detail: raw.substring(0, 400) }];
  }
}

function revokeShare(shareId) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/shares', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ shareId: ${shareId} }),
        credentials: 'include'
      }).then(function(r) { return r.json(); })
        .then(function(d) { return d.code === 0 ? 'OK' : 'ERROR: ' + d.message; })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

function getShareAfterRevoke(shareCode) {
  return ab.evalStdin(`
    (function() {
      return fetch('/v1/disk/share/' + '${shareCode}')
        .then(function(r) { return r.json(); })
        .then(function(d) {
          var active = d.data && d.data.isActive;
          return 'status=' + d.code + ' active=' + active;
        })
        .catch(function(e) { return 'ERR: ' + e.message; });
    })()
  `);
}

describe('T24: 协议层端到端 — 上传/分享/下载/预览/撤销', () => {
  ab.closeAll();
  ab.login('user001', 'test123');
  ab.waitMs(2000);

  // T24.1 — upload markdown via fetch
  const uploadResult = uploadMarkdown();
  ab.waitMs(1500);
  const uploadOk = uploadResult.includes('OK:');
  assertCondition(uploadOk, 'T24.1: fetch 上传 markdown 成功', uploadResult);
  const fileId = (uploadResult.match(/id=(\d+)/) || [])[1];

  if (!fileId) {
    step('T24.2-9: 上传失败，跳过剩余步骤', false, 'fileId 未获取到');
    ab.closeBrowser();
    return;
  }

  // T24.2 — create share via fetch
  const createResult = createShare(fileId, 't24pass');
  ab.waitMs(1500);
  const createOk = createResult.includes('OK:');
  assertCondition(createOk, 'T24.2: fetch 创建分享成功', createResult);
  const shareCode = (createResult.match(/code=([a-zA-Z0-9]+)/) || [])[1];
  const shareId = (createResult.match(/id=(\d+)/) || [])[1];

  if (!shareCode || !shareId) {
    step('T24.3-9: 分享创建失败，跳过剩余步骤', false, 'shareCode/shareId 未获取到');
    deleteFile(fileId);
    ab.closeBrowser();
    return;
  }

  // T24.3-8 — exercise protocol stack in one browser round-trip.
  // Each entry is { step, ok, detail } from the browser; we just match by
  // step name and surface ok/detail.
  const stack = exerciseProtocolStack(fileId, shareCode, 't24pass');
  ab.waitMs(3000);

  const findByStep = (name) => stack.find((s) => s.step === name) || { ok: false, detail: 'missing' };
  const publicGet = findByStep('publicGET');
  step('T24.3: 公开 GET /v1/disk/share/{code} 返回 200 + active=true', publicGet.ok, publicGet.detail);

  const access = findByStep('access');
  step('T24.4: POST /v1/disk/share/access 提取码验证通过', access.ok, access.detail);

  const downloadJSON = findByStep('downloadJSON');
  step('T24.5: POST /v1/disk/share/download → downloadToken 链路', downloadJSON.ok, downloadJSON.detail);

  const bytes = findByStep('bytes');
  step('T24.6: 下载文件内容与上传 bytes 完全一致', bytes.ok, bytes.detail);

  const previewJSON = findByStep('previewJSON');
  step('T24.7: GET /v1/disk/preview/{id} 返回 fileType=markdown', previewJSON.ok, previewJSON.detail);

  const previewHTML = findByStep('previewHTML');
  step('T24.8: GET /v1/disk/preview/{id}/html 返回 raw bytes + CSP/X-Content-Type-Options', previewHTML.ok, previewHTML.detail);

  // T24.9 — revoke share. evalStdin JSON-stringifies the result, so 'OK'
  // comes back as '"OK"' — compare with .includes, not strict equality.
  const revokeResult = revokeShare(shareId);
  ab.waitMs(1000);
  step('T24.9: DELETE /v1/disk/shares 撤销成功', revokeResult.includes('OK'), revokeResult);

  // T24.10 — verify share inaccessible after revoke
  const afterRevoke = getShareAfterRevoke(shareCode);
  ab.waitMs(1000);
  const revokedOk = afterRevoke.includes('active=false') || !afterRevoke.includes('active=true');
  step('T24.10: 撤销后分享 active=false 或不可访问', revokedOk, afterRevoke);

  ab.screenshot('t24-protocol-stack');
  ab.closeBrowser();
});
