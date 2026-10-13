import { http } from '@/lib/http.ts';

export const getEdid = () => http.get('/api/vm/edid');
export const getCustomEdidList = () => http.get('/api/vm/edid/custom');
export const updateEdid = (edid: string) => http.post('/api/vm/edid', { edid });
export const deleteEdid = (edid: string) => http.post('/api/vm/edid/delete', { edid });
export const uploadEdid = (data: FormData) => http.post('/api/vm/edid/upload', data);
export const getTimezone = () => http.get('/api/vm/timezone');
export const setTimezone = (timezone: string) => http.post('/api/vm/timezone', { timezone });
export const getTimeStatus = () => http.get('/api/vm/time/status');
export const syncTime = () => http.post('/api/vm/time/sync');
