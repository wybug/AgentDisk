import apiClient from './client';
import type { ApiResponse, UserDisk } from './types';

export const spaceApi = {
  get: () => apiClient.get<never, ApiResponse<UserDisk>>('/v1/disk/space').then(r => r.data),
};
