import apiClient from './client';
import type { UserDisk } from './types';

export const spaceApi = {
  get: () => apiClient.get<any, { code: number; message: string; data: UserDisk }>('/v1/disk/space').then(r => r.data),
};
