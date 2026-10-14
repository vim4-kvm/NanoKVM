import { useEffect, useState } from 'react';
import { Divider, InputNumber, message, Select } from 'antd';
import { useAtom, useAtomValue } from 'jotai';
import { useTranslation } from 'react-i18next';

import * as api from '@/api/vim4-stream';
import { videoModeAtom, videoParametersAtom } from '@/jotai/screen';

import { Edid } from './edid-settings';
import { Resolution } from './resolution';

export const VideoSettings = () => {
  const { t } = useTranslation();
  const mode = useAtomValue(videoModeAtom);
  const [settings, setSettings] = useAtom(videoParametersAtom);
  const [busy, setBusy] = useState(false);
  const [notice, context] = message.useMessage();
  useEffect(() => {
    api
      .getVideoSettings()
      .then((rsp) => {
        if (rsp.code === 0)
          setSettings((old) => ({ ...old, ...rsp.data, rateControlMode: rsp.data.rateControl }));
      })
      .catch((error) => notice.error(String(error)));
  }, []);
  async function update(key: keyof typeof settings, value: number | string) {
    if (busy || settings[key] === value) return;
    setBusy(true);
    try {
      const rsp =
        key === 'rateControlMode'
          ? await api.setRateControlMode(String(value))
          : key === 'fps'
            ? await api.setFps(Number(value))
            : key === 'gop'
              ? await api.setGop(Number(value))
              : await api.setQuality(Number(value));
      if (rsp.code !== 0) throw new Error(rsp.msg);
      setSettings((old) => ({ ...old, [key]: value }));
    } catch (error) {
      notice.error(String(error));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      {context}
      <div className="text-base">{t('settings.screen.title')}</div>
      <Divider className="opacity-50" />
      <div className="flex flex-col space-y-8">
        <Resolution variant="settings" />
        <Edid />
        {mode !== 'mjpeg' && (
          <>
            <div className="flex items-center justify-between">
              <span>{t('settings.screen.rateControlMode.title')}</span>
              <Select
                value={settings.rateControlMode}
                disabled={busy}
                style={{ width: 160 }}
                options={[
                  { value: 'cbr', label: 'CBR' },
                  { value: 'vbr', label: 'VBR' }
                ]}
                onChange={(value) => update('rateControlMode', value)}
              />
            </div>
            <div className="flex items-center justify-between">
              <span title={t('settings.screen.bitrate.higher')}>{t('settings.screen.bitrate.title')} (1000–60000 Kbps)</span>
              <InputNumber
                min={1000}
                max={60000}
                step={1000}
                disabled={busy}
                value={settings.bitrate}
                onChange={(value) => {
                  if (value !== null) void update('bitrate', value);
                }}
              />
            </div>
            <div className="flex items-center justify-between">
              <span>GOP</span>
              <InputNumber
                min={1}
                max={200}
                disabled={busy}
                value={settings.gop}
                onChange={(value) => {
                  if (value !== null) void update('gop', value);
                }}
              />
            </div>
          </>
        )}
        <div className="flex items-center justify-between">
          <span>{t('screen.fps')}</span>
          <Select
            value={settings.fps}
            disabled={busy}
            style={{ width: 160 }}
            options={[0, ...(mode === 'mjpeg' ? [5, 10, 15, 30, 60] : [24, 30, 60])].map((fps) => ({
              value: fps,
              label: fps ? `${fps} fps` : t('screen.auto')
            }))}
            onChange={(value) => update('fps', value)}
          />
        </div>
        {mode === 'mjpeg' && (
          <span className="text-xs text-neutral-500">{t('screen.vim4.mjpegLimit')}</span>
        )}
      </div>
    </>
  );
};
