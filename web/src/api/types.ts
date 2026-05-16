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
  resType: 'file' | 'folder';
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
  resourceId: number;
  resType: 'file' | 'folder';
  permission: 'owner' | 'read' | 'write' | 'delete';
  createdAt: string;
  updatedAt: string;
}

export interface PreviewResult {
  fileType: 'markdown' | 'code' | 'image' | 'text' | 'binary';
  url: string;
}

export interface CreateFolderRequest {
  parentId: number;
  folderName: string;
}

export interface CreateShareRequest {
  resourceId: number;
  resType: 'file' | 'folder';
  extractCode?: string;
  maxVisit?: number;
  expireHours?: number;
}

export interface GrantPermissionRequest {
  agentId: string;
  resourceId: number;
  resType: 'file' | 'folder';
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
