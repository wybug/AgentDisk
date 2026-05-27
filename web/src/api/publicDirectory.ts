import apiClient from './client';

export const publicDirectoryApi = {
  listVisible: () =>
    apiClient.get('/v1/disk/public-directories'),
  get: (id: number) =>
    apiClient.get(`/v1/disk/public-directories/${id}`),
  listSubFolders: (id: number) =>
    apiClient.get(`/v1/disk/public-directories/${id}/folders`),
};
