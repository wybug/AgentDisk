const { execFileSync, execSync } = require('child_process');
const path = require('path');

const SESSION = 'agentdisk-test';
const BASE_URL = 'http://localhost:5173';
const GATEWAY_URL = 'http://localhost:3000';
const SCREENSHOT_DIR = path.join(__dirname, '..', 'screenshots');

function ab(...args) {
  try {
    return execFileSync('agent-browser', ['--session', SESSION, ...args], {
      encoding: 'utf-8',
      timeout: 30000,
      maxBuffer: 10 * 1024 * 1024,
    }).trim();
  } catch (e) {
    throw new Error(`agent-browser ${args.join(' ')} failed:\n${(e.stderr || '').toString().trim() || e.message}`);
  }
}

function abQuiet(...args) {
  try {
    return execFileSync('agent-browser', ['--session', SESSION, ...args], {
      encoding: 'utf-8',
      timeout: 30000,
      maxBuffer: 10 * 1024 * 1024,
    }).trim();
  } catch {
    return '';
  }
}

function open(url) {
  return ab('open', url);
}

function snapshot(opts = '') {
  const extra = opts.split(' ').filter(Boolean);
  return ab('snapshot', '-i', '-c', ...extra);
}

function snapshotFull() {
  return ab('snapshot');
}

function click(ref) {
  return ab('click', ref);
}

function fill(ref, value) {
  return ab('fill', ref, value);
}

function type(ref, value) {
  return ab('type', ref, value);
}

function press(key) {
  return ab('press', key);
}

function waitMs(ms) {
  return ab('wait', String(ms));
}

function waitElement(ref) {
  return ab('wait', ref);
}

function waitText(text) {
  return ab('wait', '--text', text);
}

function waitUrl(pattern) {
  return ab('wait', '--url', pattern);
}

function waitLoad(type = 'networkidle') {
  return ab('wait', '--load', type);
}

function getUrl() {
  return ab('get', 'url');
}

function getTitle() {
  return ab('get', 'title');
}

function getText(ref) {
  return ab('get', 'text', ref);
}

function getAttr(ref, attr) {
  return ab('get', 'attr', ref, attr);
}

function findAndClick(text) {
  return ab('find', 'text', text, 'click');
}

function findAndFill(label, value) {
  return ab('find', 'text', label, 'fill', value);
}

function screenshot(name) {
  const filepath = path.join(SCREENSHOT_DIR, `${name}.png`);
  try {
    ab('screenshot', filepath);
    return filepath;
  } catch {
    return null;
  }
}

function screenshotAnnotated(name) {
  const filepath = path.join(SCREENSHOT_DIR, `${name}.png`);
  try {
    ab('screenshot', '--annotate', filepath);
    return filepath;
  } catch {
    return null;
  }
}

function scrollDown(px = 500) {
  return ab('scroll', 'down', String(px));
}

function scrollUp(px = 500) {
  return ab('scroll', 'up', String(px));
}

function setViewport(w, h) {
  return ab('set', 'viewport', String(w), String(h));
}

function evalStdin(code) {
  try {
    return execFileSync('agent-browser', ['--session', SESSION, 'eval', '--stdin'], {
      input: code,
      encoding: 'utf-8',
      timeout: 15000,
    }).trim();
  } catch {
    return '';
  }
}

function tabNew(url) {
  return ab('tab', 'new', url);
}

function tab(id) {
  return ab('tab', id);
}

function tabList() {
  return ab('tab');
}

function tabClose(id) {
  return ab('tab', 'close', id);
}

function closeBrowser() {
  try {
    execFileSync('agent-browser', ['--session', SESSION, 'close'], { encoding: 'utf-8', timeout: 10000 });
  } catch { /* ignore */ }
}

function closeAll() {
  try {
    execSync('agent-browser close --all', { encoding: 'utf-8', timeout: 5000, killSignal: 'SIGKILL' });
  } catch { /* ignore */ }
}

function login(userId = 'user001', password = 'test123') {
  open(BASE_URL);
  waitMs(4000);
  waitLoad('networkidle');

  let snap = snapshot();
  const userIdRef = findRefByPlaceholder(snap, '用户 ID');
  const passwordRef = findRefByPlaceholder(snap, '密码');
  // Button text has a space: "登 录"
  const loginBtnRef = findRefByRole(snap, 'button', '登 录') || findRefByText(snap, '登 录');

  if (!userIdRef || !passwordRef || !loginBtnRef) {
    screenshot('login-page-debug');
    throw new Error(`Login form not found. userId=${userIdRef}, pwd=${passwordRef}, btn=${loginBtnRef}`);
  }

  fill(userIdRef, userId);
  fill(passwordRef, password);
  click(loginBtnRef);

  // Wait for authorize page
  waitMs(3000);
  waitLoad('networkidle');

  // Click approve via eval (onclick may not fire from agent-browser click)
  evalStdin('approve()');
  waitMs(6000);
  waitLoad('networkidle');
}

function logout() {
  findAndClick('退出登录');
  waitMs(1000);
}

function findRefByPlaceholder(snap, placeholder) {
  // Format: textbox "用户 ID" [required, ref=e3]
  const escaped = escapeRegex(placeholder);
  const re = new RegExp(`${escaped}.*?ref=e(\\d+)`, 'i');
  const m = snap.match(re);
  return m ? `@e${m[1]}` : null;
}

function findRefByRole(snap, role, text) {
  // Format: button "登 录" [ref=e5]
  const escaped = escapeRegex(text);
  const re = new RegExp(`${role}[^\\n]*"${escaped}"[^\\n]*ref=e(\\d+)`, 'i');
  const m = snap.match(re);
  return m ? `@e${m[1]}` : null;
}

function findRefByText(snap, text) {
  // Format: button "folder-add 新建文件夹" [ref=e9]  OR  link "test-folder" [ref=e36]
  const escaped = escapeRegex(text);
  const re = new RegExp(`"[^"]*${escaped}[^"]*"[^\\n]*ref=e(\\d+)`);
  const m = snap.match(re);
  return m ? `@e${m[1]}` : null;
}

function findRefBySelector(snap, selector) {
  const re = new RegExp(`@e(\\d+).*${escapeRegex(selector)}`);
  const m = snap.match(re);
  return m ? `@e${m[1]}` : null;
}

function pageContainsText(text) {
  const snap = snapshotFull();
  return snap.includes(text);
}

function escapeRegex(str) {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// Find the button ref inside a table row that contains rowText
function findRowButtonRef(snap, rowText, btnText) {
  const lines = snap.split('\n');
  let inTargetRow = false;
  let rowIndent = 0;
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    // Detect row start
    if (line.match(/^[^a-zA-Z]*row\s/) && line.includes(rowText)) {
      inTargetRow = true;
      rowIndent = (line.match(/^\s*/) || [''])[0].length;
      continue;
    }
    if (inTargetRow) {
      const currentIndent = (line.match(/^\s*/) || [''])[0].length;
      // If we're back at same or lower indent level as the row, we're out of the row
      if (line.trim().length > 0 && currentIndent <= rowIndent) {
        inTargetRow = false;
        continue;
      }
      // Look for the button with btnText
      const escaped = escapeRegex(btnText);
      const re = new RegExp(`button[^\\n]*"${escaped}"[^\\n]*ref=e(\\d+)`);
      const m = line.match(re);
      if (m) return `@e${m[1]}`;
      // Also try generic format: button "text" [ref=eN]
      const re2 = new RegExp(`button\\s+"[^"]*${escaped}[^"]*"\\s+\\[ref=e(\\d+)\\]`);
      const m2 = line.match(re2);
      if (m2) return `@e${m2[1]}`;
    }
  }
  return null;
}

// React-friendly helpers: use JS to click buttons and fill inputs
function jsClickBtn(text) {
  return evalStdin(`
    (function() {
      var btns = document.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) {
        if (btns[i].textContent.includes('${text}')) { btns[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
}

function jsClickLink(text) {
  return evalStdin(`
    (function() {
      var links = document.querySelectorAll('a');
      for (var i = 0; i < links.length; i++) {
        if (links[i].textContent.includes('${text}')) { links[i].click(); return 'clicked'; }
      }
      return 'not found';
    })()
  `);
}

function jsFill(placeholder, value) {
  return evalStdin(`
    (function() {
      var input = null;
      var inputs = document.querySelectorAll('input[type="text"], input:not([type])');
      for (var i = 0; i < inputs.length; i++) {
        if (inputs[i].placeholder && inputs[i].placeholder.indexOf('${placeholder}') !== -1) {
          input = inputs[i]; break;
        }
      }
      if (!input) return 'input not found for: ${placeholder}';
      var nativeSet = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
      nativeSet.call(input, '${value}');
      input.dispatchEvent(new Event('input', { bubbles: true }));
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return 'filled: ${value}';
    })()
  `);
}

function jsClickElement(selector) {
  return evalStdin(`
    (function() {
      var el = document.querySelector('${selector}');
      if (el) { el.click(); return 'clicked'; }
      return 'not found';
    })()
  `);
}

module.exports = {
  SESSION, BASE_URL, GATEWAY_URL, SCREENSHOT_DIR,
  ab, abQuiet,
  open, snapshot, snapshotFull,
  click, fill, type, press,
  waitMs, waitElement, waitText, waitUrl, waitLoad,
  getUrl, getTitle, getText, getAttr,
  findAndClick, findAndFill,
  screenshot, screenshotAnnotated,
  scrollDown, scrollUp, setViewport,
  evalStdin,
  tabNew, tab, tabList, tabClose,
  closeBrowser, closeAll,
  login, logout,
  findRefByPlaceholder, findRefByRole, findRefByText, findRefBySelector,
  findRowButtonRef,
  pageContainsText,
  jsClickBtn, jsClickLink, jsFill, jsClickElement,
};
