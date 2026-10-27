// RFC 6381 profile/level comes from this stream's SPS, including Main10 and
// level changes after source or global HDR mapping changes.
export function hevcCodecFromAnnexB(data: Uint8Array): string | null {
  const startCode = (i: number) =>
    data[i] === 0 && data[i + 1] === 0
      ? data[i + 2] === 1
        ? 3
        : data[i + 2] === 0 && data[i + 3] === 1
          ? 4
          : 0
      : 0;
  for (let i = 0; i + 5 < data.length; i++) {
    const size = startCode(i);
    if (!size || ((data[i + size] >> 1) & 63) !== 33) continue;
    const rbsp: number[] = [];
    let zeros = 0;
    for (let j = i + size + 2; j < data.length && !startCode(j); j++) {
      const byte = data[j];
      if (zeros >= 2 && byte === 3) {
        zeros = 0;
        continue;
      }
      rbsp.push(byte);
      zeros = byte === 0 ? zeros + 1 : 0;
    }
    if (rbsp.length < 13) throw new Error('Truncated HEVC SPS');
    const profile = rbsp[1];
    let compatibility = 0;
    for (let bit = 0; bit < 32; bit++) {
      if (rbsp[2 + (bit >> 3)] & (1 << (7 - (bit & 7)))) compatibility += 2 ** bit;
    }
    const constraints = rbsp.slice(6, 12);
    while (constraints.length && constraints[constraints.length - 1] === 0) constraints.pop();
    const space = ['', 'A', 'B', 'C'][profile >> 6];
    const tier = profile & 32 ? 'H' : 'L';
    const suffix = constraints.map((byte) => '.' + byte.toString(16).toUpperCase()).join('');
    return `hev1.${space}${profile & 31}.${compatibility.toString(16).toUpperCase()}.${tier}${rbsp[12]}${suffix}`;
  }
  return null;
}
