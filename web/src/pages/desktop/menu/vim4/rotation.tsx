import { Popover } from 'antd';
import { useAtom } from 'jotai';
import { CheckIcon, RotateCwIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { videoRotationAtom } from '@/jotai/screen';

export const Rotation = () => {
  const { t } = useTranslation();
  const [rotation, setRotation] = useAtom(videoRotationAtom);
  const content = (
    <>
      {([0, 90, 180, 270] as const).map((value) => (
        <div
          key={value}
          className="flex cursor-pointer items-center rounded py-1.5 pl-1 pr-5 hover:bg-neutral-700/70"
          onClick={() => setRotation(value)}
        >
          <div className="h-[14px] w-[20px] text-blue-500">
            {value === rotation && <CheckIcon size={14} />}
          </div>
          {value}°
        </div>
      ))}
    </>
  );
  return (
    <Popover content={content} placement="rightTop" arrow={false}>
      <div className="flex h-[30px] cursor-pointer items-center space-x-2 rounded px-3 text-neutral-300 hover:bg-neutral-700/70">
        <RotateCwIcon size={18} />
        <span className="text-sm">{t('screen.rotation')}</span>
      </div>
    </Popover>
  );
};
