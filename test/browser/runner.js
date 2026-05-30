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
    { name: '测试网关', port: 3100 },
    { name: 'Web 前端', port: 9101 },
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
  .filter(f => f.startsWith('t') && f.endsWith('.js') && !f.includes('record'))
  .sort();

const args = process.argv.slice(2);
const skipManual = args.includes('--skip-manual');
const filter = args.find(a => !a.startsWith('--'));

let filteredTests = filter
  ? testFiles.filter(f => f.includes(filter))
  : testFiles;

if (skipManual) {
  filteredTests = filteredTests.filter(f => {
    const content = fs.readFileSync(path.join(testDir, f), 'utf-8');
    return !content.includes('MANUAL_TEST: true');
  });
  console.log(`\x1b[33m  跳过需要人工配合的测试 (--skip-manual)\x1b[0m`);
}

if (filteredTests.length === 0) {
  console.log('\n\x1b[33m没有匹配的测试文件\x1b[0m\n');
  process.exit(0);
}

checkServices();

// T01 前清空数据库，确保 init-status 测试从干净状态开始
const needsClean = filteredTests.some(f => f.startsWith('t01'));
if (needsClean) {
  console.log('\n\x1b[1m清空管理控制台数据...\x1b[0m');
  try {
    const projectRoot = path.resolve(__dirname, '../..');
    execSync(`go run scripts/clean_admin/main.go -config config.yaml`, {
      encoding: 'utf-8',
      timeout: 30000,
      cwd: projectRoot,
      stdio: 'inherit',
    });
  } catch (e) {
    console.log('\x1b[33m  警告: 数据库清理失败，T01 初始化状态测试可能被跳过\x1b[0m');
  }
}

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
      timeout: testName.includes('record') ? 180000 : 180000,
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
