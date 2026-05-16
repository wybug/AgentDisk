export type FileCategory = 'markdown' | 'code' | 'image' | 'text' | 'binary' | 'video' | 'audio' | 'pdf' | 'archive';

const EXT_MAP: Record<string, FileCategory> = {
  '.md': 'markdown',
  '.markdown': 'markdown',
  '.js': 'code', '.ts': 'code', '.tsx': 'code', '.jsx': 'code',
  '.py': 'code', '.go': 'code', '.java': 'code', '.c': 'code', '.cpp': 'code',
  '.h': 'code', '.rs': 'code', '.rb': 'code', '.php': 'code', '.swift': 'code',
  '.kt': 'code', '.sh': 'code', '.bash': 'code', '.zsh': 'code',
  '.css': 'code', '.scss': 'code', '.less': 'code', '.html': 'code', '.xml': 'code',
  '.json': 'code', '.yaml': 'code', '.yml': 'code', '.toml': 'code',
  '.sql': 'code', '.proto': 'code', '.graphql': 'code',
  '.jpg': 'image', '.jpeg': 'image', '.png': 'image', '.gif': 'image',
  '.svg': 'image', '.webp': 'image', '.bmp': 'image', '.ico': 'image',
  '.mp4': 'video', '.mov': 'video', '.avi': 'video', '.mkv': 'video', '.webm': 'video',
  '.mp3': 'audio', '.wav': 'audio', '.flac': 'audio', '.aac': 'audio', '.ogg': 'audio',
  '.pdf': 'pdf',
  '.zip': 'archive', '.tar': 'archive', '.gz': 'archive', '.rar': 'archive', '.7z': 'archive',
  '.txt': 'text', '.log': 'text', '.csv': 'text', '.conf': 'text', '.ini': 'text',
  '.env': 'text', '.gitignore': 'text',
};

export function classifyFile(fileName: string): FileCategory {
  const idx = fileName.lastIndexOf('.');
  if (idx === -1) return 'binary';
  const ext = fileName.slice(idx).toLowerCase();
  return EXT_MAP[ext] || 'binary';
}

export function getFileIcon(fileName: string): string {
  const cat = classifyFile(fileName);
  switch (cat) {
    case 'markdown': return 'file-text';
    case 'code': return 'code';
    case 'image': return 'file-image';
    case 'video': return 'video-camera';
    case 'audio': return 'audio';
    case 'pdf': return 'file-pdf';
    case 'archive': return 'file-zip';
    case 'text': return 'file-text';
    default: return 'file';
  }
}

export function isPreviewable(fileName: string): boolean {
  const cat = classifyFile(fileName);
  return ['markdown', 'code', 'image', 'text', 'pdf'].includes(cat);
}
