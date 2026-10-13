import { useEffect, useState } from 'react';
import { useAtomValue, useSetAtom } from 'jotai';
import { MonitorIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { getVideoSettings } from '@/api/vim4-stream';
import { updateScreen } from '@/api/vm';
import * as ls from '@/lib/localstorage';
import { isVIM4 } from '@/lib/platform';
import { resolutionAtom, videoModeAtom, videoParametersAtom } from '@/jotai/screen.ts';
import { MenuItem } from '@/components/menu-item.tsx';

import { Edid } from '../vim4/edid';
import { Resolution as CaptureResolution } from '../vim4/resolution';
import { Rotation } from '../vim4/rotation';
import { getQualityMap, getScreenType } from './constants.ts';
import { Fps } from './fps';
import { FrameDetect } from './frame-detect';
import { Gop } from './gop.tsx';
import { Quality } from './quality';
import { Reset } from './reset.tsx';
import { Resolution } from './resolution';
import { Scale } from './scale';
import { VideoMode } from './video-mode.tsx';

export const Screen = () => {
  const { t } = useTranslation();

  const videoMode = useAtomValue(videoModeAtom);
  const resolution = useAtomValue(resolutionAtom);
  const isMjpeg = videoMode === 'mjpeg';
  const setParameters = useSetAtom(videoParametersAtom);
  const parameters = useAtomValue(videoParametersAtom);
  const [fps, setFps] = useState(30);
  const [quality, setQuality] = useState(2);
  const [gop, setGop] = useState(30);

  useEffect(() => {
    if (isVIM4) {
      getVideoSettings()
        .then((rsp) => {
          if (rsp.code !== 0) return;
          setFps(rsp.data.fps);
          setGop(rsp.data.gop);
          setParameters((old) => ({ ...old, ...rsp.data, rateControlMode: rsp.data.rateControl }));
        })
        .catch(console.error);
      return;
    }
    const screenType = getScreenType(videoMode);
    if (screenType !== null) {
      updateScreen('type', screenType);
    }
    updateScreen('resolution', resolution!.height);
    updateQuality();
    updateFps();
    updateGop();
  }, []);

  async function updateQuality() {
    const cookieQuality = ls.getQuality();
    if (!cookieQuality) return;

    const key = cookieQuality >= 1 && cookieQuality <= 4 ? cookieQuality : 2;
    const qualityMap = getQualityMap(videoMode);
    const value = qualityMap?.get(key);
    if (value === undefined) return;

    const rsp = await updateScreen('quality', value);
    if (rsp.code === 0) {
      setQuality(key);
    }
  }

  async function updateFps() {
    const cookieFps = ls.getFps();
    if (!cookieFps) return;

    const rsp = await updateScreen('fps', cookieFps);
    if (rsp.code === 0) {
      setFps(cookieFps);
    }
  }

  async function updateGop() {
    const cookieGop = ls.getGop();
    if (!cookieGop) return;

    const rsp = await updateScreen('gop', cookieGop);
    if (rsp.code === 0) {
      setGop(cookieGop);
    }
  }

  const content = (
    <div className="flex flex-col space-y-1">
      <VideoMode />
      {isVIM4 ? (
        <>
          <CaptureResolution />
          <Edid />
        </>
      ) : (
        <Resolution />
      )}
      <Quality quality={quality} setQuality={setQuality} />
      <Fps
        fps={isVIM4 ? parameters.fps : fps}
        setFps={(value) => {
          setFps(value);
          if (isVIM4) setParameters((old) => ({ ...old, fps: value }));
        }}
      />
      <Scale />
      {isVIM4 && <Rotation />}
      {!isMjpeg && (
        <Gop
          gop={isVIM4 ? parameters.gop : gop}
          setGop={(value) => {
            setGop(value);
            if (isVIM4) setParameters((old) => ({ ...old, gop: value }));
          }}
        />
      )}
      {isMjpeg && <FrameDetect />}
      <Reset />
    </div>
  );

  return <MenuItem title={t('screen.title')} icon={<MonitorIcon size={18} />} content={content} />;
};
