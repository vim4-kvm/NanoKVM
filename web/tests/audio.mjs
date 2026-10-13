import assert from 'node:assert/strict';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const page = await browser.newPage();
const messages = [];
const errors = [];
let socket;
page.on('pageerror', (error) => errors.push(error.message));
await page.addInitScript(() => {
  const Original = window.RTCPeerConnection;
  window.RTCPeerConnection = class extends Original {
    constructor(...args) { super(...args); window.audioTestPeer = this; }
  };
});
await page.routeWebSocket('**/api/stream/audio/webrtc', (connection) => {
  socket = connection;
  connection.onMessage((data) => messages.push(JSON.parse(data)));
});
try {
  await page.goto(`${process.env.TEST_URL || 'http://127.0.0.1:5179'}/tests/audio-harness.html`);
  await page.waitForFunction(() => window.audioTestPeer);
  assert(!messages.some((m) => m.event === 'audio-offer'), 'wait for server ICE settings');
  const servers = [{ urls: ['stun:127.0.0.1:9'] }];
  socket.send(JSON.stringify({ event: 'ice-servers', data: JSON.stringify(servers) }));
  await page.waitForFunction(() => window.audioTestPeer.localDescription?.type === 'offer');
  assert.deepEqual(await page.evaluate(() => window.audioTestPeer.getConfiguration().iceServers.map((server) => server.urls)), servers.map((server) => server.urls));
  const offer = await page.evaluate(() => window.audioTestPeer.localDescription.sdp);
  assert(offer.includes('opus/48000/2'), 'offer supports stereo Opus');
  assert(offer.includes('a=sendrecv'), 'offer supports HDMI playback and microphone');
  assert.deepEqual(errors, []);
  console.log('Audio signaling passed: configured ICE, Opus sendrecv offer.');
} catch (error) {
  console.error({ errors, events: messages.map((message) => message.event) });
  throw error;
} finally { await browser.close(); }
