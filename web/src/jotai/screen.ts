import { atom } from 'jotai';
import { atomWithStorage } from 'jotai/utils';

import { ControlRegionMode, InputRegion, Resolution } from '@/types';
import type { VideoRotation } from '@/lib/video-transform';

export const isHdmiEnabledAtom = atom(true);

// video mode
// direct: stream H.264 over HTTP
// h264: stream H.264 over WebRTC
// mjpeg: stream JPEG over HTTP
export const videoModeAtom = atom('');

export const videoScaleAtom = atom<number>(1.0);
export const videoRotationAtom = atomWithStorage<VideoRotation>('nanokvm-video-rotation', 0);

export const videoVolumeAtom = atom(0);
export const videoParametersAtom = atom({
  fps: 60,
  bitrate: 12000,
  gop: 120,
  quality: 80,
  rateControlMode: 'cbr'
});

// browser screen resolution
export const resolutionAtom = atom<Resolution | null>(null);

// currently effective absolute mouse input region
export const inputRegionAtom = atom<InputRegion | null>(null);
export const manualInputRegionAtom = atom<InputRegion | null>(null);
export const manualRegionsAtom = atom<InputRegion[]>([]);
export const selectedManualRegionAtom = atom<string>('');
export const selectedOriginalResolutionAtom = atom<string>('');

// device-level control region mode; disabled by default
export const controlRegionModeAtom = atom<ControlRegionMode>('off');

// show the live input-region selection overlay
export const inputRegionSelectingAtom = atom(false);
