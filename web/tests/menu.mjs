import assert from 'node:assert/strict';

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const page = await browser.newPage({ viewport: { width: 1300, height: 900 } });
const errors = [];
const writes = [];
let authentication = 'disable';
page.on('pageerror', (error) => errors.push(error.message));
await page.route(/^http:\/\/[^/]+\/api\//, (route) => {
  const req = route.request();
  if (req.method() !== 'GET') writes.push(req.url());
  const path = new URL(req.url()).pathname;
  if (path === '/api/platform') {
    return route.fulfill({ json: { platform: 'vim4', authentication } });
  }
  const data = path.endsWith('/settings')
    ? {
        width: 3840,
        height: 2160,
        fps: 60,
        gop: 120,
        bitrate: 12000,
        quality: 80,
        rateControl: 'cbr'
      }
    : path.endsWith('/version')
      ? { current: '2.5.1', latest: '2.5.1' }
      : path.endsWith('/edid')
        ? { current: '4k60', presets: [], custom: false }
        : {};
  return route.fulfill({ json: { code: 0, data } });
});
try {
  await page.goto(`${process.env.TEST_URL || 'http://127.0.0.1:5179'}/tests/menu-harness.html`);
  await page.locator('svg.lucide-settings').click();
  assert.equal(await page.getByText('Account', { exact: true }).count(), 0,
    'disabled authentication hides account, password and logout controls');
  await page.getByText('Screen', { exact: true }).last().click();
  await page.getByRole('combobox').first().waitFor();
  assert(
    (await page.getByText('3840 × 2160', { exact: false }).count()) > 0,
    'capture dropdown labels resolution'
  );
  assert(
    (await page.getByText('60', { exact: false }).count()) > 0,
    'capture dropdown labels frame rate'
  );
  assert(
    (await page.getByText('MCP Service', { exact: true }).count()) > 0,
    'native MCP settings retained'
  );
  assert.deepEqual(writes, [], 'opening menus must not overwrite persisted capture settings');
  assert.deepEqual(errors, []);
  authentication = 'enable';
  const platformResponse = page.waitForResponse('**/api/platform');
  await page.reload();
  await platformResponse;
  await page.locator('svg.lucide-settings').click();
  await page.getByText('Account', { exact: true }).waitFor();
  console.log('Menu checks passed: native settings/MCP, capture dropdown, no startup writes.');
} finally {
  await browser.close();
}
