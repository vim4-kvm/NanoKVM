import ReactDOM from 'react-dom/client';

import '../src/i18n';
import '../src/assets/styles/index.css';

import { Wifi } from '../src/pages/desktop/menu/settings/network/wifi';

ReactDOM.createRoot(document.getElementById('root')!).render(<Wifi />);
