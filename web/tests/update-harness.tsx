import ReactDOM from 'react-dom/client';
import '../src/i18n';
import { Update } from '../src/pages/desktop/menu/settings/update';

ReactDOM.createRoot(document.getElementById('root')!).render(<Update setIsLocked={() => {}} />);
