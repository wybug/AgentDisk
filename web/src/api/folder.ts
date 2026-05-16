import apiClient from './client';
import type { DiskFolder, CreateFolderRequest } from './types';

export interface AncestorItem {
  id: number;
  folderName: string;
}

export const folderApi = {
  create: (data: CreateFolderRequest) =>
    apiClient.post('/v1/disk/folders', data),

  list: (parentId: number = 0) =>
    apiClient.get<any, { code: number; message: string; data: DiskFolder[] }>('/v1/disk/folders', { params: { parentId } }).then(r => r.data),

  getById: (id: number) =>
    apiClient.get<any, { code: number; message: string; data: DiskFolder }>(`/v1/disk/folders/${id}`).then(r => r.data),

  getAncestors: (id: number) =>
    apiClient.get<any, { code: number; message: string; data: AncestorItem[] }>(`/v1/disk/folders/${id}/ancestors`).then(r => r.data),

  delete: (id: number) =>
    apiClient.delete(`/v1/disk/folders/${id}`),

  rename: (id: number, folderName: string) =>
    apiClient.put(`/v1/disk/folders/${id}`, { folderName }),
};
