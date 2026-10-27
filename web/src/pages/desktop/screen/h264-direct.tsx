import { useEffect, useRef } from 'react';
import clsx from 'clsx';
import { useAtomValue } from 'jotai';

import { getBaseUrl } from '@/lib/service.ts';
import { mouseStyleAtom } from '@/jotai/mouse';

import DirectWorker from './direct.worker.ts?worker';
import { ScreenViewport } from './viewport.tsx';

export const H264Direct = ({ codec = 'h264' }: { codec?: 'h264' | 'h265' }) => {
  const mouseStyle = useAtomValue(mouseStyleAtom);

  const canvasRef = useRef<HTMLCanvasElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const screenRef = useRef<HTMLDivElement>(null);
  const workerRef = useRef<Worker | null>(null);

  useEffect(() => {
    if (!window.VideoDecoder) {
      console.log('Error: WebCodecs API not supported.');
      return;
    }
    if (!canvasRef.current) {
      return;
    }

    const worker = new DirectWorker();
    workerRef.current = worker;

    const offscreen = canvasRef.current.transferControlToOffscreen();
    const Generator = (
      window as unknown as {
        MediaStreamTrackGenerator?: new (options: {
          kind: string;
        }) => MediaStreamTrack & { writable: WritableStream<VideoFrame> };
      }
    ).MediaStreamTrackGenerator;
    const track = Generator ? new Generator({ kind: 'video' }) : null;
    const writer = track?.writable.getWriter();
    if (track && videoRef.current) videoRef.current.srcObject = new MediaStream([track]);
    const showHdr = (enabled: boolean) => {
      if (videoRef.current) videoRef.current.style.display = enabled ? 'block' : 'none';
      if (canvasRef.current) canvasRef.current.style.display = enabled ? 'none' : 'block';
    };
    const url = `${getBaseUrl('ws')}/api/stream/${codec}/direct`;
    worker.onmessage = (
      event: MessageEvent<{ type?: string; width?: number; height?: number; frame?: VideoFrame }>
    ) => {
      const { type, width, height } = event.data;
      if (type === 'hdr-frame' && event.data.frame) {
        const frame = event.data.frame;
        if (screenRef.current) {
          screenRef.current.dataset.mediaWidth = String(frame.displayWidth);
          screenRef.current.dataset.mediaHeight = String(frame.displayHeight);
        }
        showHdr(true);
        void videoRef.current?.play().catch(() => {});
        const written = writer ? writer.write(frame) : Promise.resolve();
        void written.catch(console.error).finally(() => {
          frame.close();
          worker.postMessage({ type: 'frame-presented' });
        });
        return;
      }
      if (type === 'hdr-mode') {
        showHdr(false);
        return;
      }
      if (type !== 'frame-size' || !width || !height || !screenRef.current) {
        return;
      }

      screenRef.current.dataset.mediaWidth = String(width);
      screenRef.current.dataset.mediaHeight = String(height);
    };
    worker.postMessage({ type: 'h264', codec, nativeHdr: !!track, canvas: offscreen, url }, [
      offscreen
    ]);

    return () => {
      worker.postMessage({ type: 'stop' });
      worker.terminate();
      void writer?.abort().catch(() => {});
      track?.stop();
      if (videoRef.current) videoRef.current.srcObject = null;
    };
  }, []);

  return (
    <ScreenViewport>
      <div
        id="screen"
        data-direct-screen
        ref={screenRef}
        className={clsx('block touch-none select-none', mouseStyle)}
      >
        <canvas ref={canvasRef} className="h-full w-full" style={{ objectFit: 'inherit' }} />
        <video
          ref={videoRef}
          className="h-full w-full"
          style={{ display: 'none', objectFit: 'inherit' }}
          autoPlay
          muted
          playsInline
        />
      </div>
    </ScreenViewport>
  );
};
