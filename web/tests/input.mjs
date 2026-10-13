import assert from 'node:assert/strict';

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const page = await browser.newPage({ viewport: { width: 1000, height: 720 } });
const reports = [];
const errors = [];
page.on('pageerror', (error) => errors.push(error.message));
await page.route(/^http:\/\/[^/]+\/api\//, (route) =>
  route.fulfill({ json: { code: 0, data: { shortcuts: [], key: '', mode: 'simultaneous' } } })
);
await page.routeWebSocket('**/api/ws', (socket) =>
  socket.onMessage((data) => {
    if (typeof data !== 'string') reports.push([...data]);
  })
);
const mouse = () => reports.filter((r) => r[0] === 2);
const neutral = (r) => r[1] === 0;
try {
  await page.goto(`${process.env.TEST_URL || 'http://127.0.0.1:5179'}/tests/input-harness.html`);
  await page.waitForFunction(
    () =>
      window.inputHarness && document.querySelector('#screen')?.getBoundingClientRect().width > 0
  );
  await page.waitForTimeout(150);
  await page.mouse.move(450, 300);
  await page.mouse.down();
  await page.waitForTimeout(30);
  assert.equal(mouse().at(-1)[1], 1, 'absolute press reaches host');
  await page.evaluate(() => window.dispatchEvent(new Event('blur')));
  await page.waitForTimeout(30);
  assert(neutral(mouse().at(-1)), 'blur releases a held button');
  await page.mouse.up();
  await page.evaluate(() => window.inputHarness.rotation(90));
  await page.waitForTimeout(50);
  await page.mouse.move(450, 100);
  await page.waitForTimeout(30);
  const rotated = mouse().at(-1);
  const x = rotated[2] | (rotated[3] << 8),
    y = rotated[4] | (rotated[5] << 8);
  assert(x < 10000 && Math.abs(y - 16384) < 100, `rotated input maps into source frame: ${x},${y}`);
  await page.evaluate(() => window.inputHarness.crop());
  await page.waitForTimeout(50);
  await page.mouse.move(450, 300);
  await page.mouse.down();
  await page.mouse.move(980, 680);
  await page.mouse.up();
  await page.waitForTimeout(30);
  assert(neutral(mouse().at(-1)), 'outside release clears cropped drag');
  await page.evaluate(() => window.inputHarness.mode('relative'));
  await page.waitForTimeout(50);
  await page.evaluate(() => {
    const target = document.querySelector('#screen');
    const touch = new Touch({ identifier: 1, target, clientX: 450, clientY: 300 });
    target.dispatchEvent(
      new TouchEvent('touchstart', { touches: [touch], changedTouches: [touch], bubbles: true })
    );
    window.dispatchEvent(new Event('blur'));
    target.dispatchEvent(
      new TouchEvent('touchend', { touches: [], changedTouches: [touch], bubbles: true })
    );
  });
  await page.waitForTimeout(30);
  assert(neutral(mouse().at(-1)), 'canceled touch cannot become a click');
  await page.keyboard.down('ArrowRight');
  await page.evaluate(() => window.dispatchEvent(new Event('blur')));
  await page.waitForTimeout(30);
  const key = reports.filter((r) => r[0] === 1).at(-1);
  assert(
    key.slice(1).every((byte) => byte === 0),
    'blur releases held keys'
  );
  assert.deepEqual(errors, []);
  console.log(
    'Input regression checks passed: blur, rotation, crop, outside release, canceled touch, keyboard.'
  );
} finally {
  await browser.close();
}
