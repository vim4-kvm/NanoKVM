import ReactDOM from 'react-dom/client';

import '../src/i18n';
import '../src/assets/styles/index.css';

import { getDefaultStore } from 'jotai';

import { mouseModeAtom } from '../src/jotai/mouse';
import { controlRegionModeAtom, inputRegionAtom, videoRotationAtom } from '../src/jotai/screen';
import { client } from '../src/lib/websocket';
import { Keyboard } from '../src/pages/desktop/keyboard';
import { Mouse } from '../src/pages/desktop/mouse';
import { initializeMouseInputLifecycle } from '../src/pages/desktop/mouse/lifecycle';
import { ScreenViewport } from '../src/pages/desktop/screen/viewport';

const store = getDefaultStore();
initializeMouseInputLifecycle();
client.connect();
Object.assign(window, {
  inputHarness: {
    mode: (mode: string) => store.set(mouseModeAtom, mode),
    rotation: (rotation: 0 | 90 | 180 | 270) => store.set(videoRotationAtom, rotation),
    crop: () => {
      store.set(inputRegionAtom, {
        frameWidth: 1920,
        frameHeight: 1080,
        left: 240,
        top: 0,
        width: 1440,
        height: 1080
      });
      store.set(controlRegionModeAtom, 'manual');
    }
  }
});
ReactDOM.createRoot(document.getElementById('root')!).render(
  <div style={{ width: 900, height: 600 }}>
    <ScreenViewport>
      <canvas
        id="screen"
        width={1920}
        height={1080}
        data-media-width="1920"
        data-media-height="1080"
      />
    </ScreenViewport>
    <Mouse />
    <Keyboard />
  </div>
);
