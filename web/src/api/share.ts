import apiClient from './client';
import type { DiskShare, CreateShareRequest } from './types';

export const shareApi = {
  create: (data: CreateShareRequest) =>
    apiClient.post('/v1/disk/shares', data),

  list: () =>
    apiClient.get<any, { code: number; message: string; data: DiskShare[] }>('/v1/disk/shares').then(r => r.data),

  revoke: (shareId: number) =>
    apiClient.delete('/v1/disk/shares', { data: { shareId } }),

  getPublic: (code: string) =>
    apiClient.get<any, { code: number; message: string; data: DiskShare }>(`/v1/disk/share/${code}`).then(r => r.data),

  accessPublic: (code: string, extractCode?: string) =>
    apiClient.post('/v1/disk/share/access', { code, extractCode }),
};
