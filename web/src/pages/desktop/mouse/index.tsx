import { useAtomValue } from 'jotai';

import { mouseModeAtom } from '@/jotai/mouse.ts';
import { inputRegionSelectingAtom } from '@/jotai/screen';

import { Absolute } from './absolute.tsx';
import { Relative } from './relative.tsx';

export const Mouse = () => {
  const mouseMode = useAtomValue(mouseModeAtom);
  const selecting = useAtomValue(inputRegionSelectingAtom);
  if (selecting) return null;

  return <>{mouseMode === 'relative' ? <Relative /> : <Absolute />}</>;
};
