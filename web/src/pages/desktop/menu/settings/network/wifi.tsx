import { useEffect, useState } from 'react';
import { LockOutlined, WifiOutlined } from '@ant-design/icons';
import { Button, Input, Modal, Switch } from 'antd';
import { CheckIcon, RefreshCwIcon, WifiIcon, WifiPenIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import * as api from '@/api/network.ts';
import { isVIM4 } from '@/lib/platform.ts';

export const Wifi = () => {
  const { t } = useTranslation();

  const [isSupported, setIsSupported] = useState(false);
  const [isAPMode, setIsAPMode] = useState(false);
  const [connectedWiFi, setConnectedWifi] = useState('');

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [ssid, setSsid] = useState('');
  const [password, setPassword] = useState('');
  const [status, setStatus] = useState<'' | 'connecting' | 'disconnecting'>('');
  const [message, setMessage] = useState('');
  const [networks, setNetworks] = useState<api.WifiNetwork[]>([]);
  const [isScanning, setIsScanning] = useState(false);
  const [scanFailed, setScanFailed] = useState(false);
  const [scanRevision, setScanRevision] = useState(0);
  const [selectedNetwork, setSelectedNetwork] = useState<api.WifiNetwork | null>(null);

  useEffect(() => {
    if (!isVIM4 || !isModalOpen) return;
    let cancelled = false;
    setIsScanning(true);
    setScanFailed(false);
    setNetworks([]);

    async function scan() {
      try {
        const rsp = await api.scanWifi();
        if (rsp.code !== 0) throw new Error('Wi-Fi scan failed');
        // Connections use SSID, so show each name once, with its strongest signal.
        const strongest = new Map<string, api.WifiNetwork>();
        for (const network of rsp.data.wifiList as api.WifiNetwork[]) {
          const previous = strongest.get(network.ssid);
          if (!previous || network.signal > previous.signal) {
            strongest.set(network.ssid, network);
          }
        }
        if (!cancelled) {
          setNetworks([...strongest.values()].sort((a, b) => b.signal - a.signal));
        }
      } catch {
        if (!cancelled) setScanFailed(true);
      } finally {
        if (!cancelled) setIsScanning(false);
      }
    }

    void scan();
    return () => {
      cancelled = true;
    };
  }, [isModalOpen, scanRevision]);

  useEffect(() => {
    getWiFi();
  }, []);

  async function getWiFi() {
    try {
      const rsp = await api.getWiFi();
      if (rsp.code !== 0) {
        console.log(rsp.msg);
        return;
      }

      setIsSupported(!!rsp.data?.supported);
      setIsAPMode(!!rsp.data?.apMode);

      if (rsp.data?.connected && rsp.data?.ssid) {
        setConnectedWifi(rsp.data.ssid);
      } else {
        setConnectedWifi('');
      }
    } catch {
      /* empty */
    }
  }

  async function connect() {
    setMessage('');

    const requiresPassword =
      !isVIM4 ||
      (selectedNetwork && selectedNetwork.security !== '' && selectedNetwork.security !== '--');
    if (!ssid || (requiresPassword && !password)) return;

    if (status !== '') return;
    setStatus('connecting');

    try {
      const rsp = await api.connectWifi(ssid, password);
      if (rsp.code !== 0) {
        console.log(rsp.msg);
        setMessage(t('settings.network.wifi.failed'));
        getWiFi();
        return;
      }

      setConnectedWifi(ssid);
      setIsModalOpen(false);
    } catch {
      setMessage(t('settings.network.wifi.failed'));
      getWiFi();
    } finally {
      setStatus('');
    }
  }

  async function disconnect(enable: boolean) {
    if (enable || status !== '') return;

    setStatus('disconnecting');

    try {
      const rsp = await api.disconnectWifi();
      if (rsp.code !== 0) {
        console.log(rsp.msg);
        return;
      }

      setConnectedWifi('');
    } catch (err) {
      console.log(err);
    } finally {
      setStatus('');
    }
  }

  function openModal() {
    setSelectedNetwork(null);
    setSsid('');
    setPassword('');
    setMessage('');
    setIsModalOpen(true);
  }

  function closeModal() {
    if (status !== '') return;
    setIsModalOpen(false);
  }

  if (!isSupported) {
    return <></>;
  }

  if (isAPMode) {
    return (
      <div className="flex items-center justify-between">
        <div className="flex flex-col space-y-1">
          <span>{t('settings.network.wifi.title')}</span>
          <span className="text-xs text-neutral-500">{t('settings.network.wifi.apMode')}</span>
        </div>

        <Button shape="round" size="small" disabled>
          <div className="flex items-center justify-center px-1.5">
            <WifiIcon size={16} />
          </div>
        </Button>
      </div>
    );
  }

  return (
    <>
      <div className="flex items-center justify-between">
        <div className="flex flex-col space-y-1">
          <span>{t('settings.network.wifi.title')}</span>
          <span className="text-xs text-neutral-500">
            {connectedWiFi ? connectedWiFi : t('settings.network.wifi.description')}
          </span>
        </div>

        <Button
          type={connectedWiFi ? 'primary' : 'default'}
          shape="round"
          size="small"
          onClick={openModal}
          aria-label={t('settings.network.wifi.description')}
        >
          <div className="flex items-center justify-center px-1.5">
            <WifiIcon size={16} />
          </div>
        </Button>
      </div>

      <Modal
        closable={false}
        open={isModalOpen}
        centered={true}
        onOk={connect}
        onCancel={closeModal}
        okText={t('settings.network.wifi.joinBtn')}
        cancelText={t('settings.network.wifi.cancelBtn')}
        confirmLoading={status === 'connecting'}
        okButtonProps={{ disabled: status === 'disconnecting' }}
      >
        {/* title */}
        <div className="flex items-center space-x-5">
          <div className="h-[64px] w-[64px]">
            <WifiPenIcon size={64} className="text-blue-400" />
          </div>

          {!connectedWiFi ? (
            <div className="flex flex-col">
              <span className="text-lg font-bold">{t('settings.network.wifi.connect')}</span>
              <span className="text-xs text-neutral-400">
                {t(
                  isVIM4 ? 'settings.network.wifi.scanHint' : 'settings.network.wifi.connectDesc1'
                )}
              </span>
            </div>
          ) : (
            <div className="flex justify-center">
              <div className="flex w-[300px] justify-between rounded-lg bg-neutral-800">
                <div className="flex w-full justify-between p-3">
                  <span>{connectedWiFi}</span>
                  <Switch
                    value={!!connectedWiFi}
                    loading={status === 'disconnecting'}
                    onChange={disconnect}
                  />
                </div>
              </div>
            </div>
          )}
        </div>

        {/* form */}
        <div className="flex flex-col items-center space-y-3 py-6">
          {isVIM4 && (
            <div className="w-[300px] max-w-full space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-sm text-neutral-300">
                  {t('settings.network.wifi.nearby')}
                </span>
                <Button
                  shape="round"
                  size="small"
                  icon={<RefreshCwIcon size={14} />}
                  loading={isScanning}
                  disabled={status !== ''}
                  onClick={() => setScanRevision((value) => value + 1)}
                >
                  {t('settings.network.wifi.scan')}
                </Button>
              </div>
              {scanFailed && (
                <div role="alert" className="text-sm text-red-500">
                  {t('settings.network.wifi.scanFailed')}
                </div>
              )}
              {!isScanning && !scanFailed && networks.length === 0 && (
                <div className="text-sm text-neutral-500">
                  {t('settings.network.wifi.scanEmpty')}
                </div>
              )}
              <div className="max-h-48 space-y-0.5 overflow-y-auto rounded-lg bg-neutral-800/50">
                {networks.map((network) => (
                  <button
                    key={network.ssid}
                    type="button"
                    disabled={status !== ''}
                    aria-pressed={selectedNetwork?.ssid === network.ssid}
                    className="flex min-h-[40px] w-full cursor-pointer items-center gap-2 rounded border-0 bg-transparent px-3 py-2 text-left text-sm text-neutral-300 transition-colors hover:bg-neutral-700/50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-400 disabled:cursor-not-allowed disabled:opacity-50 aria-pressed:bg-neutral-700/50"
                    onClick={() => {
                      setSelectedNetwork(network);
                      setSsid(network.ssid);
                      setPassword('');
                      setMessage('');
                    }}
                  >
                    {selectedNetwork?.ssid === network.ssid ? (
                      <CheckIcon size={16} className="shrink-0 text-blue-400" />
                    ) : (
                      <WifiIcon size={16} className="shrink-0 text-neutral-400" />
                    )}
                    <span className="flex min-w-0 flex-col space-y-0.5">
                      <span className="break-all">{network.ssid}</span>
                      <span className="text-xs text-neutral-500">
                        {network.signal}% ·{' '}
                        {network.security && network.security !== '--'
                          ? network.security
                          : t('settings.network.wifi.openNetwork')}
                      </span>
                    </span>
                  </button>
                ))}
              </div>
            </div>
          )}
          <Input
            value={ssid}
            disabled={status !== ''}
            style={{ width: '300px' }}
            prefix={<WifiOutlined />}
            placeholder={t('settings.network.wifi.ssid')}
            onChange={(e) => {
              setSsid(e.target.value);
              setSelectedNetwork(null);
              setPassword('');
              setMessage('');
            }}
          />
          <Input.Password
            value={password}
            disabled={
              status !== '' ||
              !!(
                selectedNetwork &&
                (!selectedNetwork.security || selectedNetwork.security === '--')
              )
            }
            style={{ width: '300px' }}
            prefix={<LockOutlined />}
            placeholder={t('settings.network.wifi.password')}
            onChange={(e) => setPassword(e.target.value)}
          />

          {!!message && <span className="text-sm text-red-500">{message}</span>}
        </div>
      </Modal>
    </>
  );
};
