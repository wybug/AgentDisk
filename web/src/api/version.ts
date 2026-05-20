import apiClient from './client';
import type { ApiResponse, DiskFileVersion } from './types';

export const versionApi = {
  list: (fileId: number) =>
    apiClient.get<never, ApiResponse<DiskFileVersion[]>>('/v1/disk/versions', { params: { fileId } }).then(r => r.data),

  rollback: (fileId: number, version: number) =>
    apiClient.post('/v1/disk/versions/rollback', { fileId, version }),
};
