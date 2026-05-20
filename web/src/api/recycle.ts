import apiClient from './client';
import type { ApiResponse, DiskRecycleBin } from './types';

export const recycleApi = {
  list: () =>
    apiClient.get<never, ApiResponse<DiskRecycleBin[]>>('/v1/disk/recycle').then(r => r.data),

  restore: (recycleId: number) =>
    apiClient.post('/v1/disk/recycle/restore', { recycleId }),

  deletePermanent: (recycleId: number) =>
    apiClient.delete('/v1/disk/recycle', { data: { recycleId } }),
};
