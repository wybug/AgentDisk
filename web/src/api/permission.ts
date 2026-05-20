import apiClient from './client';
import type { ApiResponse, DiskPermission, GrantPermissionRequest } from './types';

export const permissionApi = {
  grant: (data: GrantPermissionRequest) =>
    apiClient.post('/v1/disk/permissions', data),

  list: (params?: { agentId?: string; resourceId?: number; resType?: string }) =>
    apiClient.get<never, ApiResponse<DiskPermission[]>>('/v1/disk/permissions', { params }).then(r => r.data),

  revoke: (data: { agentId?: string; agentGroupId?: string; resourceId?: number; resType?: string; resourcePath?: string }) =>
    apiClient.delete('/v1/disk/permissions', { data }),

  check: (params: { agentId?: string; agentGroupId?: string; resourceId: number; resType: string; permission: string }) =>
    apiClient.get<never, ApiResponse<{ allowed: boolean }>>('/v1/disk/permissions/check', { params }).then(r => r.data),
};
