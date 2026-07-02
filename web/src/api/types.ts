export interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

export interface UserDisk {
  id: number;
  userId: string;
  totalQuota: number;
  usedQuota: number;
  rootFolder: string;
  createdAt: string;
  updatedAt: string;
}

export interface DiskFolder {
  id: number;
  userId: string;
  parentId: number;
  folderName: string;
  fullPath: string;
  sortOrder: number;
  isDeleted: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface DiskFile {
  id: number;
  userId: string;
  folderId: number;
  fileName: string;
  fileSize: number;
  fileType: string;
  ossKey: string;
  md5: string;
  version: number;
  isDeleted: boolean;
  sourceAgent: string;
  isArtifact: boolean;
  tags: string;
  createdAt: string;
  updatedAt: string;
}

export interface DiskFileVersion {
  id: number;
  fileId: number;
  userId: string;
  version: number;
  ossKey: string;
  fileSize: number;
  md5: string;
  snapshotBy: string;
  createdAt: string;
}

export interface DiskShare {
  id: number;
  userId: string;
  resourceId: number;
  resType: 'file' | 'folder' | 'bundle';
  shareCode: string;
  extractCode: string;
  maxVisit: number;
  visitCount: number;
  expireAt: string;
  isActive: boolean;
  createdAt: string;
}

export interface DiskRecycleBin {
  id: number;
  userId: string;
  resourceId: number;
  resType: 'file' | 'folder';
  resName: string;
  originalPath: string;
  deletedBy: string;
  expireAt: string;
  createdAt: string;
}

export interface DiskPermission {
  id: number;
  userId: string;
  agentId: string;
  agentGroupId: string;
  resourceId: number;
  resType: 'file' | 'folder';
  resourcePath: string;
  permission: 'owner' | 'read' | 'write' | 'delete';
  createdAt: string;
  updatedAt: string;
}

export interface PreviewResult {
  fileType: 'markdown' | 'code' | 'image' | 'text' | 'binary' | 'html';
  url?: string;
  content?: string;
}

export interface CreateFolderRequest {
  parentId: number;
  folderName: string;
}

export interface CreateShareRequest {
  resourceId: number;
  resType: 'file' | 'folder' | 'bundle';
  extractCode?: string;
  maxVisit?: number;
  expireHours?: number;
}

export interface GrantPermissionRequest {
  agentId?: string;
  agentGroupId?: string;
  resourceId?: number;
  resType?: 'file' | 'folder';
  resourcePath?: string;
  permission: 'owner' | 'read' | 'write' | 'delete';
}

export interface RollbackRequest {
  fileId: number;
  version: number;
}

export interface DownloadTokenResult {
  downloadToken: string;
  expiresIn: number;
}

// ──────────────────────────────── OKF v0.1 ────────────────────────────────

export interface OkfBundle {
  bundleId: number;
  publicDirectoryId: number;
  okfVersion: string;
  rootIndexFileId: number;
  title: string;
  description: string;
  status: 'active' | 'archived';
  nodeCount: number;
  edgeCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface OkfNode {
  nodeId: number;
  bundleId: number;
  fileId: number;
  relPath: string;
  type: string;
  title: string;
  description: string;
  tags: string[];
  timestamp: string;
  hasBrokenLink: boolean;
  extra: Record<string, unknown>;
  contentHash: string;
  createdAt: string;
  updatedAt: string;
}

export interface OkfEdge {
  edgeId: number;
  srcNodeId: number;
  dstNodeId: number;
  dstRelPath: string;
  linkText: string;
  srcLine: number;
  linkKind: 'bundle' | 'external' | 'anchor';
  dstExists: boolean;
}

export interface OkfTypeCount {
  type: string;
  count: number;
}

export interface OkfBrokenLink {
  srcNodeId: number;
  srcRelPath: string;
  dstRelPath: string;
  srcLine: number;
  linkText: string;
  linkKind: 'bundle' | 'external' | 'anchor';
  reason: string;
}

export interface OkfBundleStats {
  nodeCount: number;
  edgeTotal: number;
  edgeLive: number;
  edgeBroken: number;
  types: OkfTypeCount[];
}

export interface OkfSearchPage {
  nodes: OkfNode[];
  nextCursor: number;
}

export interface OkfScanReport {
  scannedNodes: number;
  brokenCount: number;
  scannedAt: string;
}

export interface OkfIndexRegenResult {
  indexVersion: number;
  regeneratedAt: string;
}

export interface OkfRegisterBundleRequest {
  publicDirectoryId: number;
}

export interface OkfSearchRequest {
  query: string;
  bundleId?: number;
  type?: string;
  limit?: number;
  cursor?: number;
}

export interface OkfSubgraphRequest {
  bundleId: number;
  types?: string[];
  maxNodes?: number;
}

export interface OkfNodesResult {
  nodes: OkfNode[];
}

export interface OkfGraphResult {
  nodes: OkfNode[];
  edges: OkfEdge[];
}

export interface OkfBrokenLinksPage {
  brokenLinks: OkfBrokenLink[];
  nextCursor: number;
}

