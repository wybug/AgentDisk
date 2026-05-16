import apiClient from './client';
import type { DiskFileVersion } from './types';

export const versionApi = {
  list: (fileId: number) =>
    apiClient.get<any, { code: number; message: string; data: DiskFileVersion[] }>('/v1/disk/versions', { params: { fileId } }).then(r => r.data),

  rollback: (fileId: number, version: number) =>
    apiClient.post('/v1/disk/versions/rollback', { fileId, version }),
};
