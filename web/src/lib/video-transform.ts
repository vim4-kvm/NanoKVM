export type VideoRotation = 0 | 90 | 180 | 270;

export function inverseRotatePoint(x: number, y: number, rotation: VideoRotation) {
  switch (rotation) {
    case 90:
      return { x: y, y: 1 - x };
    case 180:
      return { x: 1 - x, y: 1 - y };
    case 270:
      return { x: 1 - y, y: x };
    default:
      return { x, y };
  }
}

export function inverseRotateDelta(x: number, y: number, rotation: VideoRotation) {
  switch (rotation) {
    case 90:
      return { x: y, y: -x };
    case 180:
      return { x: -x, y: -y };
    case 270:
      return { x: -y, y: x };
    default:
      return { x, y };
  }
}
