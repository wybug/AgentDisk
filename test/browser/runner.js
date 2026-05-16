#!/usr/bin/env node
const { execSync } = require('child_process');
const path = require('path');
const fs = require('fs');

const testDir = path.join(__dirname, 'tests');
const screenshotDir = path.join(__dirname, 'screenshots');

// 确保截图目录存在
if (!fs.existsSync(screenshotDir)) {
  fs.mkdirSync(screenshotDir, { recursive: true });
}

// 检查服务是否运行
function checkServices() {
  console.log('\n\x1b[1m检查服务状态...\x1b[0m');
  const services = [
    { name: '后端 API', port: 9100 },
    { name: '测试网关', port: 3000 },
    { name: 'Web 前端', port: 5173 },
  ];

  let allRunning = true;
  for (const s of services) {
    try {
      const result = execSync(`lsof -ti:${s.port}`, { encoding: 'utf-8' }).trim();
      if (result) {
        console.log(`  \x1b[32m✓\x1b[0m ${s.name} (:${s.port}) 运行中`);
      } else {
        console.log(`  \x1b[31m✗\x1b[0m ${s.name} (:${s.port}) 未运行`);
        allRunning = false;
      }
    } catch {
      console.log(`  \x1b[31m✗\x1b[0m ${s.name} (:${s.port}) 未运行`);
      allRunning = false;
    }
  }

  if (!allRunning) {
    console.log('\n\x1b[31m请先启动所有服务: bash scripts/dev.sh start\x1b[0m\n');
    process.exit(1);
  }
}

// 获取测试文件列表
const testFiles = fs.readdirSync(testDir)
  .filter(f => f.startsWith('t') && f.endsWith('.js'))
  .sort();

const filter = process.argv[2];
const filteredTests = filter
  ? testFiles.filter(f => f.includes(filter))
  : testFiles;

if (filteredTests.length === 0) {
  console.log('\n\x1b[33m没有匹配的测试文件\x1b[0m\n');
  process.exit(0);
}

checkServices();

console.log(`\n\x1b[1m\x1b[36m═════════════════════════════════════════════════\x1b[0m`);
console.log(`\x1b[1m\x1b[36m  AgentDisk Browser Tests - ${filteredTests.length} test(s)\x1b[0m`);
console.log(`\x1b[1m\x1b[36m═════════════════════════════════════════════════\x1b[0m`);

// 关闭所有已有浏览器会话
try {
  execSync('agent-browser close --all', { encoding: 'utf-8', timeout: 10000 });
} catch {}

const results = [];

for (const testFile of filteredTests) {
  const testPath = path.join(testDir, testFile);
  const testName = testFile.replace('.js', '');

  console.log(`\n\x1b[1m--- 运行 ${testName} ---\x1b[0m`);
  const startTime = Date.now();

  try {
    execSync(`node "${testPath}"`, {
      encoding: 'utf-8',
      timeout: 120000,
      stdio: 'inherit',
    });
    const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
    results.push({ name: testName, status: 'PASS', time: elapsed });
    console.log(`\x1b[32m  ✓ ${testName} 完成 (${elapsed}s)\x1b[0m`);
  } catch (e) {
    const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
    results.push({ name: testName, status: 'FAIL', time: elapsed });
    console.log(`\x1b[31m  ✗ ${testName} 失败 (${elapsed}s)\x1b[0m`);
  }
}

// 关闭所有浏览器
try {
  execSync('agent-browser close --all', { encoding: 'utf-8', timeout: 10000 });
} catch {}

// 汇总报告
console.log('\n\n\x1b[1m\x1b[36m═════════════════════════════════════════════════\x1b[0m');
console.log('\x1b[1m\x1b[36m  Test Summary\x1b[0m');
console.log('\x1b[1m\x1b[36m═════════════════════════════════════════════════\x1b[0m\n');

let passed = 0, failed = 0;
for (const r of results) {
  const icon = r.status === 'PASS' ? '\x1b[32m✓\x1b[0m' : '\x1b[31m✗\x1b[0m';
  console.log(`  ${icon} ${r.name} (${r.time}s)`);
  if (r.status === 'PASS') passed++;
  else failed++;
}

console.log('\n\x1b[1m─────────────────────────────────────────────────\x1b[0m');
const allPassed = failed === 0;
console.log(`  Total: ${results.length}, \x1b[32m${passed} passed\x1b[0m, \x1b[31m${failed} failed\x1b[0m`);
console.log(`  Result: ${allPassed ? '\x1b[32mALL PASSED\x1b[0m' : '\x1b[31mHAS FAILURES\x1b[0m'}`);
console.log(`  Screenshots: ${screenshotDir}`);
console.log('\x1b[1m─────────────────────────────────────────────────\x1b[0m\n');

process.exit(allPassed ? 0 : 1);
