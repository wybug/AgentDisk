import type { DiskPermission } from '@/api/types';

export interface AgentConfig {
  agentId?: string;
  agentGroupId?: string;
}

export function parseAgentConfig(json: string): AgentConfig | null {
  try {
    const obj = JSON.parse(json);
    if (typeof obj !== 'object' || obj === null) return null;
    if (!obj.agentId && !obj.agentGroupId) return null;
    return {
      agentId: obj.agentId || undefined,
      agentGroupId: obj.agentGroupId || undefined,
    };
  } catch {
    return null;
  }
}

export function formatAgentTarget(perm: DiskPermission): string {
  const parts: string[] = [];
  if (perm.agentId) parts.push(perm.agentId);
  if (perm.agentGroupId) parts.push(`组:${perm.agentGroupId}`);
  return parts.join(' + ') || '-';
}

export function formatResourceTarget(perm: DiskPermission): string {
  if (perm.resourcePath) return perm.resourcePath;
  if (perm.resourceId) return `${perm.resourceId} (${perm.resType})`;
  return '-';
}

export function getAuthType(perm: DiskPermission): 'path' | 'resourceId' {
  return perm.resourcePath ? 'path' : 'resourceId';
}

export const GLOB_HELP_TEXT = '* 单级匹配 | ** 多级匹配 | *.ext 扩展名匹配';
export const AGENT_CONFIG_PLACEHOLDER = '{"agentId":"agent-001"} 或 {"agentGroupId":"group-001"}';
export const PATH_PLACEHOLDER = '/Documents/**/*.pdf';
