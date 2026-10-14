import { ConfigProvider, theme } from 'antd';
import ReactDOM from 'react-dom/client';

import '../src/i18n';
import '../src/assets/styles/index.css';

import { Wifi } from '../src/pages/desktop/menu/settings/network/wifi';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <ConfigProvider theme={{ algorithm: theme.darkAlgorithm }}>
    <div style={{ margin: '0 auto', maxWidth: 512, padding: 24 }} className="text-neutral-300">
      <Wifi />
    </div>
  </ConfigProvider>
);
