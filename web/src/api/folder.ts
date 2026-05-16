import apiClient from './client';
import type { DiskFolder, CreateFolderRequest } from './types';

export const folderApi = {
  create: (data: CreateFolderRequest) =>
    apiClient.post('/v1/disk/folders', data),

  list: (parentId: number = 0) =>
    apiClient.get<any, { code: number; message: string; data: DiskFolder[] }>('/v1/disk/folders', { params: { parentId } }).then(r => r.data),

  delete: (id: number) =>
    apiClient.delete(`/v1/disk/folders/${id}`),
};
