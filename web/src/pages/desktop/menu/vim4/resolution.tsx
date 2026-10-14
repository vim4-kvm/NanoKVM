import { useEffect } from 'react';
import { Divider, message, Popover, Select } from 'antd';
import { atom, useAtom, useAtomValue } from 'jotai';
import { CheckIcon, MonitorIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { getVideoSettings, setResolution } from '@/api/vim4-stream';
import { videoParametersAtom } from '@/jotai/screen';

const captureWidthAtom = atom(0);
const captureBusyAtom = atom(false);
const resolutions = [
  [1280, 720],
  [1920, 1080],
  [3840, 2160]
];

export const Resolution = ({ variant = 'menu' }: { variant?: 'menu' | 'settings' }) => {
  const { t } = useTranslation();
  const videoParameters = useAtomValue(videoParametersAtom);
  const frameRateLimit = Math.min(videoParameters.fps || 60, 60);
  const [width, setWidth] = useAtom(captureWidthAtom);
  const [busy, setBusy] = useAtom(captureBusyAtom);
  const [notice, context] = message.useMessage();
  useEffect(() => {
    getVideoSettings()
      .then((rsp) => {
        if (rsp.code === 0) setWidth(rsp.data.width);
      })
      .catch((err) => notice.error(String(err)));
  }, []);
  async function update(w: number, h: number) {
    if (busy || w === width) return;
    setBusy(true);
    try {
      const rsp = await setResolution(w, h);
      if (rsp.code !== 0) throw new Error(rsp.msg);
      setWidth(w);
    } catch (err) {
      notice.error(String(err));
    } finally {
      setBusy(false);
    }
  }
  if (variant === 'settings') {
    return (
      <div className="flex items-center justify-between space-x-5">
        {context}
        <div className="flex flex-col space-y-1">
          <span>{t('screen.vim4.resolution')}</span>
          <span className="text-xs text-neutral-500">{t('screen.vim4.resolutionHint')}</span>
        </div>
        <Select
          aria-label={t('screen.vim4.resolution')}
          value={width || undefined}
          style={{ width: 240, flexShrink: 0 }}
          loading={busy}
          disabled={busy}
          options={resolutions.map(([w, h]) => ({
            value: w,
            label: `${w} × ${h} · ${t('screen.vim4.frameRateLimit', { fps: frameRateLimit })}`
          }))}
          onSelect={(w) => {
            const resolution = resolutions.find(([value]) => value === w);
            if (resolution) update(resolution[0], resolution[1]);
          }}
        />
      </div>
    );
  }
  const content = (
    <>
      {resolutions.map(([w, h]) => (
        <button
          key={w}
          type="button"
          aria-pressed={w === width}
          disabled={busy}
          onClick={() => update(w, h)}
          className="flex w-full cursor-pointer select-none items-center rounded border-0 bg-transparent py-1.5 pl-1 pr-6 text-left text-inherit [font:inherit] hover:bg-neutral-700/70 disabled:cursor-wait disabled:opacity-50"
        >
          <span className="flex h-[14px] w-[20px] items-end text-blue-500">
            {w === width && <CheckIcon size={14} />}
          </span>
          <span className="flex w-[80px] items-center justify-between">
            <span>{w}</span>
            <span className="text-xs">x</span>
            <span>{h}</span>
          </span>
          <span className="pl-3">{t('screen.vim4.frameRateLimit', { fps: frameRateLimit })}</span>
        </button>
      ))}
      <Divider style={{ margin: '5px 0' }} />
      <div className="max-w-[260px] px-1 text-xs text-neutral-500">
        {t('screen.vim4.resolutionHint')}
      </div>
    </>
  );
  return (
    <>
      {context}
      <Popover content={content} placement="rightTop" arrow={false} align={{ offset: [14, 0] }}>
        <div className="flex h-[30px] cursor-pointer items-center space-x-2 rounded pl-3 pr-6 text-neutral-300 hover:bg-neutral-700/70">
          <MonitorIcon size={18} />
          <span className="select-none text-sm">{t('screen.vim4.resolution')}</span>
        </div>
      </Popover>
    </>
  );
};
