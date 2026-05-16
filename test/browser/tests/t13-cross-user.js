const { describe, step, assertCondition } = require('../lib/test-runner');
const ab = require('../lib/agent-browser');
const { execSync } = require('child_process');

const SESSION_A = 'user-a';
const SESSION_B = 'user-b';

function abA(...args) {
  return execSync(['agent-browser', '--session', SESSION_A, ...args].join(' '), { encoding: 'utf-8', timeout: 30000 }).trim();
}

function abB(...args) {
  return execSync(['agent-browser', '--session', SESSION_B, ...args].join(' '), { encoding: 'utf-8', timeout: 30000 }).trim();
}

describe('T13: 跨用户隔离', () => {
  // 清理旧 session
  try { execSync('agent-browser close --all', { encoding: 'utf-8', timeout: 10000 }); } catch {}

  // T13.1 - user001 创建文件夹和上传文件
  abA('open', ab.BASE_URL);
  try { abA('wait', '--url', '"**/login**"'); } catch {}
  try {
    let snapA = abA('snapshot', '-i', '-c');
    const userIdRef = snapA.match(/@e(\d+).*placeholder="用户 ID"/);
    const pwdRef = snapA.match(/@e(\d+).*placeholder="密码"/);
    if (userIdRef && pwdRef) {
      abA('fill', `@e${userIdRef[1]}`, 'user001');
      abA('fill', `@e${pwdRef[1]}`, 'test123');
      const btnMatch = snapA.match(/@e(\d+)\[button[^"]*\].*"登录"/);
      if (btnMatch) abA('click', `@e${btnMatch[1]}`);
    }
    abA('wait', '--url', '"**/localhost:5173**"');
    abA('wait', '--load', 'networkidle');
  } catch {}

  abA('wait', '2000');

  // 创建一个特殊名称的文件夹
  try {
    let snapA = abA('snapshot', '-i', '-c');
    const createMatch = snapA.match(/@e(\d+).*"新建文件夹"/) || snapA.match(/@e(\d+).*"新建"/);
    if (createMatch) {
      abA('click', `@e${createMatch[1]}`);
      abA('wait', '1000');
      snapA = abA('snapshot', '-i', '-c');
      const inputMatch = snapA.match(/@e(\d+).*placeholder="文件夹名称"/);
      if (inputMatch) abA('fill', `@e${inputMatch[1]}`, '张三专属');
      snapA = abA('snapshot', '-i', '-c');
      const btnMatch = snapA.match(/@e(\d+)\[button[^"]*\].*"创建"/) || snapA.match(/@e(\d+)\[button[^"]*\].*"确定"/);
      if (btnMatch) abA('click', `@e${btnMatch[1]}`);
      abA('wait', '--load', 'networkidle');
      abA('wait', '1000');
    }
  } catch {}

  const user1Data = abA('snapshot');
  const hasSpecialFolder = user1Data.includes('张三专属');
  assertCondition(hasSpecialFolder, 'T13.1: user001 创建「张三专属」文件夹成功');

  try { execSync(`agent-browser --session ${SESSION_A} close`, { encoding: 'utf-8', timeout: 10000 }); } catch {}

  // T13.2 & T13.3 - user002 登录
  abB('open', ab.BASE_URL);
  try { abB('wait', '--url', '"**/login**"'); } catch {}
  try {
    let snapB = abB('snapshot', '-i', '-c');
    const userIdRef = snapB.match(/@e(\d+).*placeholder="用户 ID"/);
    const pwdRef = snapB.match(/@e(\d+).*placeholder="密码"/);
    if (userIdRef && pwdRef) {
      abB('fill', `@e${userIdRef[1]}`, 'user002');
      abB('fill', `@e${pwdRef[1]}`, 'test123');
      const btnMatch = snapB.match(/@e(\d+)\[button[^"]*\].*"登录"/);
      if (btnMatch) abB('click', `@e${btnMatch[1]}`);
    }
    abB('wait', '--url', '"**/localhost:5173**"');
    abB('wait', '--load', 'networkidle');
  } catch {}

  abB('wait', '2000');

  // T13.4 - 验证 user002 看不到 user001 的数据
  const user2Data = abB('snapshot');
  const notVisible = !user2Data.includes('张三专属');
  assertCondition(notVisible, 'T13.4: user002 看不到 user001 的数据');
  try { execSync(`agent-browser --session ${SESSION_B} screenshot /tmp/t13-user2-view.png`, { encoding: 'utf-8' }); } catch {}

  try { execSync(`agent-browser --session ${SESSION_B} close`, { encoding: 'utf-8', timeout: 10000 }); } catch {}
});
