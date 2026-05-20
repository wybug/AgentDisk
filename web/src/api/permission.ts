import apiClient from './client';
import type { ApiResponse, DiskPermission, GrantPermissionRequest } from './types';

export const permissionApi = {
  grant: (data: GrantPermissionRequest) =>
    apiClient.post('/v1/disk/permissions', data),

  list: (params?: { agentId?: string; resourceId?: number; resType?: string }) =>
    apiClient.get<never, ApiResponse<DiskPermission[]>>('/v1/disk/permissions', { params }).then(r => r.data),

  revoke: (data: { agentId: string; resourceId: number; resType: string }) =>
    apiClient.delete('/v1/disk/permissions', { data }),
};
