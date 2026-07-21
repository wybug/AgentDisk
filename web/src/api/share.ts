import apiClient from './client';
import type { ApiResponse, DiskShare, DiskShareStats, CreateShareRequest, DownloadTokenResult } from './types';

export const shareApi = {
  create: (data: CreateShareRequest) =>
    apiClient.post<never, ApiResponse<DiskShare>>('/v1/disk/shares', data).then(r => r.data),

  list: () =>
    apiClient.get<never, ApiResponse<DiskShare[]>>('/v1/disk/shares').then(r => r.data),

  stats: (shareId: number) =>
    apiClient.get<never, ApiResponse<DiskShareStats>>(`/v1/disk/shares/${shareId}/stats`).then(r => r.data),

  revoke: (shareId: number) =>
    apiClient.delete('/v1/disk/shares', { data: { shareId } }),

  getPublic: (code: string) =>
    apiClient.get<never, ApiResponse<DiskShare>>(`/v1/disk/share/${code}`).then(r => r.data),

  accessPublic: (code: string, extractCode?: string) =>
    apiClient.post('/v1/disk/share/access', { code, extractCode }),

  downloadPublic: (code: string, resourceId: number, extractCode?: string) =>
    apiClient.post<never, ApiResponse<DownloadTokenResult>>('/v1/disk/share/download', { code, resourceId, extractCode }).then(r => r.data),
};
