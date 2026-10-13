export const QualityMap = new Map([
  [1, 100],
  [2, 80],
  [3, 60],
  [4, 50]
]);

export const BitRateMap = new Map([
  [1, 5000],
  [2, 3000],
  [3, 2000],
  [4, 1000]
]);

export function getQualityMap(videoMode: string) {
  if (videoMode === 'mjpeg') {
    return QualityMap;
  }
  if (videoMode === 'direct' || videoMode === 'h264') {
    return BitRateMap;
  }
  if (videoMode === 'h265' || videoMode === 'h265-direct') return BitRateMap;
  return null;
}

export function getScreenType(videoMode: string) {
  if (videoMode === 'h265' || videoMode === 'h265-direct') return 2;
  if (videoMode === 'mjpeg') {
    return 0;
  }
  if (videoMode === 'direct' || videoMode === 'h264') {
    return 1;
  }
  return null;
}
