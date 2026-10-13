import { useEffect, useState } from 'react';

import { getTimezone } from '@/api/vim4.ts';

import { Synchronization } from './synchronization.tsx';
import { Time } from './time.tsx';
import { Timezone } from './timezone.tsx';

export const Datetime = () => {
  const [timezone, setTimezone] = useState('');
  const timeFormat = '24H';

  useEffect(() => {
    getTimezone().then((rsp) => {
      if (rsp.code !== 0) {
        console.log(rsp.msg);
        return;
      }

      setTimezone(rsp.data.timezone);
    });
  }, []);

  return (
    <div className="flex flex-col space-y-8">
      <Time timezone={timezone} timeFormat={timeFormat} />
      <Timezone timezone={timezone} setTimezone={setTimezone} />
      <Synchronization timezone={timezone} />
    </div>
  );
};
