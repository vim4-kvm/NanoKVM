import assert from 'node:assert/strict';

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const page = await browser.newPage({ locale: 'en-US' });
const writes = [];
const errors = [];
let scanMode = 'success';
let pendingScan;
let rejectConnection = false;
const networks = [
  { ssid: 'Home:网络', bssid: '00:11:22:33:44:55', signal: 35, frequency: 2412, security: 'WPA2' },
  { ssid: 'Guest', bssid: '00:11:22:33:44:56', signal: 60, frequency: 2412, security: '--' },
  { ssid: 'Home:网络', bssid: '00:11:22:33:44:57', signal: 90, frequency: 5180, security: 'WPA2' }
];
page.on('pageerror', (error) => errors.push(error.message));
await page.route(/^http:\/\/[^/]+\/api\//, async (route) => {
  const request = route.request();
  const path = new URL(request.url()).pathname;
  if (path.endsWith('/wifi/scan')) {
    if (scanMode === 'pending') {
      pendingScan = route;
      return;
    }
    return route.fulfill({
      json:
        scanMode === 'failure'
          ? { code: -2, msg: 'Scan failed' }
          : { code: 0, data: { wifiList: scanMode === 'empty' ? [] : networks } }
    });
  }
  if (request.method() === 'POST') {
    assert.equal(path, '/api/network/wifi/connect');
    writes.push(request.postDataJSON());
    return route.fulfill({ json: { code: rejectConnection ? -2 : 0, data: {} } });
  }
  return route.fulfill({ json: { code: 0, data: { supported: true, connected: false } } });
});

const open = () => page.getByRole('button', { name: 'Configure Wi-Fi', exact: true }).click();
const home = () => page.getByRole('button', { name: /Home:网络/ });
const join = () => page.getByRole('button', { name: 'Join', exact: true }).click();
try {
  await page.goto(`${process.env.TEST_URL || 'http://127.0.0.1:5179'}/tests/wifi-harness.html`);
  await open();
  await home().waitFor();
  assert.equal(await home().count(), 1, 'merge duplicate SSIDs');
  assert.match(await home().innerText(), /90%/);
  const options = page.getByRole('button', { name: /Home:网络|Guest/ });
  assert.match(await options.first().innerText(), /Home:网络/, 'strongest first');
  await home().click();
  assert.equal(await page.getByPlaceholder('Name', { exact: true }).inputValue(), 'Home:网络');
  await page.getByPlaceholder('Password', { exact: true }).fill('secret123');
  await join();
  await page.getByRole('dialog').waitFor({ state: 'hidden' });
  assert.deepEqual(writes.at(-1), { ssid: 'Home:网络', password: 'secret123' });

  await open();
  await page.getByRole('button', { name: /Guest/ }).click();
  assert(await page.getByPlaceholder('Password', { exact: true }).isDisabled());
  await join();
  await page.getByRole('dialog').waitFor({ state: 'hidden' });
  assert.deepEqual(writes.at(-1), { ssid: 'Guest', password: '' });

  scanMode = 'failure';
  await open();
  await page.getByRole('alert').waitFor();
  await page.getByPlaceholder('Name', { exact: true }).fill('Hidden network');
  await page.getByPlaceholder('Password', { exact: true }).fill('hidden123');
  rejectConnection = true;
  await join();
  await page.getByText('Connection failed, please try again.', { exact: true }).waitFor();
  assert.deepEqual(writes.at(-1), { ssid: 'Hidden network', password: 'hidden123' });
  scanMode = 'empty';
  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await page.getByText('No networks found.', { exact: false }).waitFor();
  assert.equal(await page.getByRole('alert').count(), 0);

  scanMode = 'pending';
  const requested = page.waitForRequest((request) => request.url().endsWith('/wifi/scan'));
  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await requested;
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();
  scanMode = 'success';
  await open();
  await home().waitFor();
  await pendingScan.fulfill({ json: { code: -2, msg: 'Late failure' } });
  // Wait for the old response to be processed before checking the new dialog.
  await page.waitForLoadState('networkidle');
  assert.equal(await page.getByRole('alert').count(), 0, 'ignore responses from a closed dialog');
  assert.equal(await home().count(), 1);
  assert.deepEqual(errors, []);
  console.log(
    'Wi-Fi checks passed: scan, deduplication, selection, open/manual networks, failures, refresh and stale responses.'
  );
} finally {
  await browser.close();
}
