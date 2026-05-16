import apiClient from './client';
import type { DiskFile, PreviewResult, DownloadTokenResult } from './types';

export const fileApi = {
  upload: (file: File, folderId: number, onProgress?: (percent: number) => void) => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('folderId', String(folderId));
    return apiClient.post('/v1/disk/files/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      onUploadProgress: (e) => {
        if (e.total && onProgress) onProgress(Math.round((e.loaded * 100) / e.total));
      },
    });
  },

  list: (folderId: number) =>
    apiClient.get<any, { code: number; message: string; data: DiskFile[] }>('/v1/disk/files', { params: { folderId } }).then(r => r.data),

  get: (id: number) =>
    apiClient.get<any, { code: number; message: string; data: { file: DiskFile; url: string } }>(`/v1/disk/files/${id}`).then(r => r.data),

  update: (id: number, file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    return apiClient.put(`/v1/disk/files/${id}`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
  },

  delete: (id: number) =>
    apiClient.delete(`/v1/disk/files/${id}`),

  getDownloadToken: (id: number) =>
    apiClient.post<any, { code: number; message: string; data: DownloadTokenResult }>(`/v1/disk/files/${id}/download-token`).then(r => r.data),

  preview: (id: number) =>
    apiClient.get<any, { code: number; message: string; data: PreviewResult }>(`/v1/disk/preview/${id}`).then(r => r.data),
};
