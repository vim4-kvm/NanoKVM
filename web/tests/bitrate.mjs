import assert from 'node:assert/strict';

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const page = await browser.newPage({ viewport: { width: 1300, height: 900 } });
let bitrate = 17500;
let rejectWrite = false;
const writes = [];
const errors = [];
page.on('pageerror', (error) => errors.push(error.message));
await page.route(/^http:\/\/[^/]+\/api\//, (route) => {
  const request = route.request();
  const path = new URL(request.url()).pathname;
  if (request.method() !== 'GET') {
    writes.push({ path, data: request.postDataJSON() });
    if (rejectWrite) return route.fulfill({ json: { code: 1, msg: 'Save failed' } });
    assert.equal(path, '/api/stream/quality');
    bitrate = request.postDataJSON().quality;
  }
  return route.fulfill({
    json: {
      code: 0,
      data: {
        width: 3840,
        height: 2160,
        fps: 60,
        gop: 120,
        bitrate,
        quality: 80,
        rateControl: 'cbr'
      }
    }
  });
});

async function openQuality() {
  await page.locator('svg.lucide-monitor').click();
  await page.getByText(`Quality · ${bitrate / 1000} Mbps`, { exact: true }).hover();
  await page.getByRole('slider').waitFor();
}

try {
  await page.goto(`${process.env.TEST_URL || 'http://127.0.0.1:5179'}/tests/menu-harness.html`);
  await openQuality();
  assert.equal(await page.getByRole('slider').getAttribute('aria-valuenow'), '17500');
  assert.equal(await page.locator('button[aria-pressed="true"]').count(), 0);
  assert.deepEqual(writes, [], 'loading saved settings must not write defaults');

  const saved = page.waitForResponse((response) => response.url().endsWith('/stream/quality'));
  await page.getByRole('slider').focus();
  await page.keyboard.press('ArrowRight');
  await saved;
  await page.getByText('Quality · 17.6 Mbps', { exact: true }).waitFor();
  assert.equal(writes.length, 1);
  assert.equal(writes[0].data.quality, 17600);

  await page.reload();
  await openQuality();
  assert.equal(await page.getByRole('slider').getAttribute('aria-valuenow'), '17600');
  assert.equal(writes.length, 1, 'refresh must restore without overwriting');
  await page.getByRole('button', { name: '60 Mbps', exact: true }).click();
  await page.getByText('Quality · 60 Mbps', { exact: true }).waitFor();
  assert.equal(bitrate, 60000);

  rejectWrite = true;
  await page.getByRole('slider').focus();
  await page.keyboard.press('ArrowLeft');
  await page.getByText('Error: Save failed', { exact: true }).waitFor();
  assert.equal(await page.getByRole('slider').getAttribute('aria-valuenow'), '60000');
  assert.equal(bitrate, 60000, 'failed save must preserve the last saved bitrate');
  rejectWrite = false;
  const handle = await page.getByRole('slider').boundingBox();
  const writesBeforeDrag = writes.length;
  await page.mouse.move(handle.x + handle.width / 2, handle.y + handle.height / 2);
  await page.mouse.down();
  await page.mouse.move(handle.x - 80, handle.y + handle.height / 2, { steps: 5 });
  const draft = Number(await page.getByRole('slider').getAttribute('aria-valuenow'));
  assert(draft < 60000 && draft >= 1000);
  assert.equal(writes.length, writesBeforeDrag, 'dragging must not restart the encoder');
  const dragSaved = page.waitForResponse((response) => response.url().endsWith('/stream/quality'));
  await page.mouse.up();
  await dragSaved;
  assert.equal(bitrate, draft);
  assert.equal(writes.length, writesBeforeDrag + 1, 'release saves once');
  assert.deepEqual(errors, []);
  console.log(
    'Bitrate checks passed: custom value, slider save, refresh, preset and failure rollback.'
  );
} finally {
  await browser.close();
}
