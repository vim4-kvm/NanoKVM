import ReactDOM from 'react-dom/client';

import '../src/i18n';
import '../src/assets/styles/index.css';

import { AuthContext } from '../src/contexts/auth';
import { Screen } from '../src/pages/desktop/menu/screen';
import { Settings } from '../src/pages/desktop/menu/settings';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <AuthContext.Provider value={{ account: { username: 'admin', role: 'admin' } }}>
    <div style={{ padding: 40, display: 'flex', gap: 20 }}>
      <Settings />
      <Screen />
    </div>
  </AuthContext.Provider>
);
