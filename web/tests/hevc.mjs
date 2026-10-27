import assert from 'node:assert/strict';
import test from 'node:test';
import { hevcCodecFromAnnexB } from '../src/pages/desktop/screen/hevc.ts';

function sps(profile, compatibility, level, prefix = [0, 0, 0, 1]) {
  const rbsp = [1, profile, compatibility, 0, 0, 0, 0xb0, 0, 0, 0, 0, 0, level];
  const escaped = [];
  let zeros = 0;
  for (const byte of rbsp) {
    if (zeros === 2 && byte <= 3) { escaped.push(3); zeros = 0; }
    escaped.push(byte);
    zeros = byte === 0 ? zeros + 1 : 0;
  }
  return [...prefix, 0x42, 1, ...escaped];
}

test('Direct decoder follows SDR/HDR and level changes in the actual SPS', () => {
  for (const [profile, compatibility, level, expected] of [
    [1, 0x60, 93, 'hev1.1.6.L93.B0'],
    [2, 0x20, 150, 'hev1.2.4.L150.B0'],
    [1, 0x60, 153, 'hev1.1.6.L153.B0'],
    [2, 0x20, 153, 'hev1.2.4.L153.B0'],
  ]) {
    const au = new Uint8Array([0, 0, 1, 0x40, 1, 0x80,
      ...sps(profile, compatibility, level, [0, 0, 1]), 0, 0, 0, 1, 0x26, 1, 0x80]);
    assert.equal(hevcCodecFromAnnexB(au), expected);
  }
});

test('Missing or truncated SPS cannot configure a decoder', () => {
  assert.equal(hevcCodecFromAnnexB(new Uint8Array([0, 0, 1, 0x26, 1, 0x80])), null);
  assert.throws(() => hevcCodecFromAnnexB(new Uint8Array([0, 0, 0, 1, 0x42, 1, 1])), /Truncated/);
});
