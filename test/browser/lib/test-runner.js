const fs = require('fs');
const path = require('path');

const results = [];
let currentTest = null;
let stepIndex = 0;

function describe(name, fn) {
  currentTest = { name, steps: [], passed: 0, failed: 0 };
  stepIndex = 0;
  console.log(`\n\x1b[1m\x1b[36m▶ ${name}\x1b[0m`);
  try {
    fn();
  } catch (e) {
    step(`Setup`, false, e.message);
  }
  if (currentTest.passed + currentTest.failed > 0) {
    results.push(currentTest);
  }
}

function step(name, passed, detail = '') {
  stepIndex++;
  const icon = passed ? '\x1b[32m✓\x1b[0m' : '\x1b[31m✗\x1b[0m';
  const label = passed ? `${icon} ${stepIndex}. ${name}` : `${icon} ${stepIndex}. ${name}`;
  console.log(`  ${label}`);
  if (detail && !passed) {
    console.log(`    \x1b[31m${detail}\x1b[0m`);
  }
  if (currentTest) {
    currentTest.steps.push({ name, passed, detail });
    if (passed) currentTest.passed++;
    else currentTest.failed++;
  }
}

function expectUrlContains(pattern) {
  const { getUrl } = require('./agent-browser');
  const url = getUrl();
  const ok = url.includes(pattern);
  step(`URL contains "${pattern}"`, ok, ok ? '' : `Actual URL: ${url}`);
  return ok;
}

function expectTextExists(text) {
  const { pageContainsText } = require('./agent-browser');
  const ok = pageContainsText(text);
  step(`Page contains "${text}"`, ok, ok ? '' : `Text "${text}" not found on page`);
  return ok;
}

function expectRefNotNull(ref, label) {
  const ok = ref !== null;
  step(`${label} found`, ok, ok ? '' : `${label} not found in snapshot`);
  return ok;
}

function assertCondition(condition, label, detail = '') {
  step(label, condition, detail);
  if (!condition) throw new Error(`Assertion failed: ${label} - ${detail}`);
  return condition;
}

function printReport() {
  console.log('\n\n\x1b[1m\x1b[36m═════════════════════════════════════════════════\x1b[0m');
  console.log('\x1b[1m\x1b[36m  AgentDisk Browser Test Report\x1b[0m');
  console.log('\x1b[1m\x1b[36m═════════════════════════════════════════════════\x1b[0m\n');

  let totalPassed = 0, totalFailed = 0;
  for (const t of results) {
    const icon = t.failed === 0 ? '\x1b[32mPASS\x1b[0m' : '\x1b[31mFAIL\x1b[0m';
    console.log(`  ${icon}  ${t.name}  (${t.passed}/${t.passed + t.failed})`);
    for (const s of t.steps) {
      if (!s.passed) {
        console.log(`       \x1b[31m→ ${s.name}: ${s.detail}\x1b[0m`);
      }
    }
    totalPassed += t.passed;
    totalFailed += t.failed;
  }

  console.log('\n\x1b[1m─────────────────────────────────────────────────\x1b[0m');
  const total = totalPassed + totalFailed;
  const allPassed = totalFailed === 0;
  console.log(`  Total: ${total} steps, \x1b[32m${totalPassed} passed\x1b[0m, \x1b[31m${totalFailed} failed\x1b[0m`);
  console.log(`  Result: ${allPassed ? '\x1b[32mALL PASSED\x1b[0m' : '\x1b[31mHAS FAILURES\x1b[0m'}`);
  console.log('\x1b[1m─────────────────────────────────────────────────\x1b[0m\n');

  // Write report file
  const reportPath = path.join(__dirname, '..', 'report.md');
  let report = '# AgentDisk Browser Test Report\n\n';
  report += `**Date**: ${new Date().toISOString()}\n`;
  report += `**Result**: ${allPassed ? 'ALL PASSED' : 'HAS FAILURES'}\n`;
  report += `**Total**: ${total} steps, ${totalPassed} passed, ${totalFailed} failed\n\n`;
  report += '| Test | Status | Steps |\n|------|--------|-------|\n';
  for (const t of results) {
    const status = t.failed === 0 ? 'PASS' : 'FAIL';
    report += `| ${t.name} | ${status} | ${t.passed}/${t.passed + t.failed} |\n`;
  }
  if (totalFailed > 0) {
    report += '\n## Failed Steps\n\n';
    for (const t of results) {
      for (const s of t.steps) {
        if (!s.passed) {
          report += `- **${t.name} → ${s.name}**: ${s.detail}\n`;
        }
      }
    }
  }
  fs.writeFileSync(reportPath, report);
  console.log(`Report saved to: ${reportPath}`);

  return allPassed;
}

module.exports = { describe, step, expectUrlContains, expectTextExists, expectRefNotNull, assertCondition, printReport };
