import assert from 'node:assert/strict';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage();
  let current = '2.5.1+vim4.20261015.9';
  let latest = '2.5.1+vim4.20261015.10';
  let installs = 0;
  await page.route('**/api/application/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/application/version') return route.fulfill({ json: { code: 0, data: { current, latest } } });
    if (path === '/api/application/update') installs++;
    return route.fulfill({ json: { code: 0, data: { enabled: false, enable: false, url: '' } } });
  });
  const url = `${process.env.TEST_URL || 'http://localhost:5191'}/tests/update-harness.html`;
  for (const [installed, remote, offered] of [
    [current, latest, true],
    [latest, current, false],
    [latest, latest, false],
    ['2.5.1+vim4.20261015.99', '2.5.1+vim4.20261016.1', true],
    ['2.5.1+vim4.20261016.1020.dev.9.g49c45f2.dirty', '2.5.1+vim4.20261016.1020', false],
    ['2.5.1+vim4.20261016.1020.dev.9.g49c45f2.dirty', '2.5.1+vim4.20261016.1022', true],
    ['2.5.1+vim4.20261016.1020.dev.9.g49c45f2', '2.5.1+vim4.20261017.1', true],
  ]) {
    current = installed; latest = remote;
    await page.goto(url);
    await page.locator('.ant-result-title').waitFor();
    assert.equal(await page.locator('.ant-result-title').innerText(), offered ? `${current} -> ${latest}` : current);
  }
  assert.equal(installs, 0, 'checking versions must not start installation');
  console.log('PASS: VIM4 updates compare numeric build revisions, dates and equality');
} finally {
  await browser.close();
}
