import { useState } from 'react';
import { message, Popover, Slider } from 'antd';
import { useAtom, useAtomValue } from 'jotai';
import { CheckIcon, SquareActivityIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { setQuality as setVideoQuality } from '@/api/vim4-stream';
import { updateScreen } from '@/api/vm';
import { setQuality as setCookie } from '@/lib/localstorage.ts';
import { isVIM4 } from '@/lib/platform';
import { videoModeAtom, videoParametersAtom } from '@/jotai/screen.ts';

import { getQualityMap } from './constants.ts';

type QualityProps = {
  quality: number;
  setQuality: (quality: number) => void;
};

export const Quality = ({ quality, setQuality }: QualityProps) => {
  const { t } = useTranslation();
  const videoMode = useAtomValue(videoModeAtom);
  const [parameters, setParameters] = useAtom(videoParametersAtom);
  const [busy, setBusy] = useState(false);
  const [draftBitrate, setDraftBitrate] = useState<number | null>(null);
  const [notice, context] = message.useMessage();
  const showBitrate = isVIM4 && videoMode !== 'mjpeg';

  const qualityList = showBitrate
    ? [1, 2, 3, 5, 8, 12, 20, 30, 40, 60].map((mbps) => ({
        key: mbps * 1000,
        label: `${mbps} Mbps`
      }))
    : [
        { key: 1, label: t('screen.qualityLossless') },
        { key: 2, label: t('screen.qualityHigh') },
        { key: 3, label: t('screen.qualityMedium') },
        { key: 4, label: t('screen.qualityLow') }
      ];
  const selected = showBitrate
    ? parameters.bitrate
    : isVIM4
      ? [...(getQualityMap(videoMode) ?? [])].find(([, value]) => value === parameters.quality)?.[0]
      : quality;

  async function update(key: number) {
    if (busy) return;
    const qualityMap = getQualityMap(videoMode);
    const value = showBitrate ? key : qualityMap?.get(key);
    if (value === undefined) {
      return;
    }

    setBusy(true);
    try {
      const rsp = isVIM4 ? await setVideoQuality(value) : await updateScreen('quality', value);
      if (rsp.code !== 0) throw new Error(rsp.msg);
      if (isVIM4) {
        setParameters((old) => ({ ...old, [showBitrate ? 'bitrate' : 'quality']: value }));
      } else {
        setQuality(key);
        setCookie(key);
      }
    } catch (error) {
      notice.error(String(error));
    } finally {
      setBusy(false);
      setDraftBitrate(null);
    }
  }

  const content = (
    <>
      {showBitrate && (
        <div className="w-[180px] px-1 pb-2">
          <div className="text-center">{(draftBitrate ?? parameters.bitrate) / 1000} Mbps</div>
          <Slider
            min={1000}
            max={60000}
            step={100}
            value={draftBitrate ?? parameters.bitrate}
            disabled={busy}
            ariaLabelForHandle="Bitrate (Mbps)"
            tooltip={{ formatter: (value) => `${(value ?? 0) / 1000} Mbps` }}
            onChange={setDraftBitrate}
            onChangeComplete={update}
          />
          <div className="flex justify-between text-xs text-neutral-400">
            <span>1 Mbps</span>
            <span>60 Mbps</span>
          </div>
        </div>
      )}
      {qualityList.map((item) => (
        <button
          type="button"
          key={item.key}
          disabled={busy}
          aria-pressed={item.key === selected}
          className="flex h-[30px] w-full cursor-pointer select-none items-center rounded border-0 bg-transparent py-0 pl-1 pr-5 text-left text-inherit [font:inherit] hover:bg-neutral-700/70 disabled:opacity-50"
          onClick={() => update(item.key)}
        >
          <div className="flex h-[14px] w-[20px] items-end text-blue-500">
            {item.key === selected && <CheckIcon size={14} />}
          </div>
          <span className="flex min-w-[70px] whitespace-nowrap">{item.label}</span>
        </button>
      ))}
    </>
  );

  return (
    <>
      {context}
      <Popover content={content} placement="rightTop" arrow={false} align={{ offset: [14, 0] }}>
        <div className="flex h-[30px] cursor-pointer items-center space-x-2 rounded px-3 text-neutral-300 hover:bg-neutral-700/70">
          <SquareActivityIcon size={18} />
          <span className="select-none text-sm">
            {t('screen.quality')}
            {showBitrate ? ` · ${parameters.bitrate / 1000} Mbps` : ''}
          </span>
        </div>
      </Popover>
    </>
  );
};
