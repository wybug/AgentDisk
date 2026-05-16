import apiClient from './client';
import type { DiskFile } from './types';

export const tagApi = {
  bind: (fileId: number, tagName: string) =>
    apiClient.post('/v1/disk/tags/bind', { fileId, tagName }),

  unbind: (fileId: number, tagName: string) =>
    apiClient.post('/v1/disk/tags/unbind', { fileId, tagName }),

  search: (tags: string) =>
    apiClient.get<any, { code: number; message: string; data: DiskFile[] }>('/v1/disk/tags/search', { params: { tags } }).then(r => r.data),
};
