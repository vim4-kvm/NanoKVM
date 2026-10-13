import { useAtomValue } from 'jotai';

import { isVIM4 } from '@/lib/platform';
import { videoModeAtom } from '@/jotai/screen.ts';

import { H264Direct } from './h264-direct.tsx';
import { H264Webrtc } from './h264-webrtc.tsx';
import { Mjpeg } from './mjpeg.tsx';
import { WebrtcAudio } from './webrtc-audio';

export const Screen = () => {
  const videoMode = useAtomValue(videoModeAtom);

  if (isVIM4) {
    return (
      <>
        {videoMode === 'mjpeg' ? (
          <Mjpeg />
        ) : videoMode === 'direct' || videoMode === 'h265-direct' ? (
          <H264Direct key={videoMode} codec={videoMode === 'h265-direct' ? 'h265' : 'h264'} />
        ) : (
          <H264Webrtc key={videoMode} codec={videoMode === 'h265' ? 'h265' : 'h264'} />
        )}
        <WebrtcAudio />
      </>
    );
  }

  if (videoMode === 'mjpeg') {
    return <Mjpeg />;
  }

  if (videoMode === 'direct') {
    return <H264Direct />;
  }

  if (videoMode === 'h264') {
    return <H264Webrtc />;
  }

  return null;
};
