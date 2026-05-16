import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { Button, Space } from 'antd';
import { FolderAddOutlined, ReloadOutlined } from '@ant-design/icons';
import { useQueryClient, useQuery } from '@tanstack/react-query';
import FileList from '@/components/file/FileList';
import FileUpload from '@/components/file/FileUpload';
import CreateFolderModal from '@/components/folder/CreateFolderModal';
import BreadcrumbNav from '@/components/layout/BreadcrumbNav';
import FilePreview from '@/components/file/FilePreview';
import VersionHistory from '@/components/file/VersionHistory';
import CreateShareModal from '@/components/share/CreateShareModal';
import TagInput from '@/components/tag/TagInput';
import { folderApi } from '@/api/folder';
import type { DiskFile } from '@/api/types';

export default function ExplorerPage() {
  const { folderId: folderIdStr } = useParams();
  const folderId = folderIdStr ? parseInt(folderIdStr, 10) : 0;
  const queryClient = useQueryClient();

  const [createFolderOpen, setCreateFolderOpen] = useState(false);
  const [previewFile, setPreviewFile] = useState<DiskFile | null>(null);
  const [versionFile, setVersionFile] = useState<DiskFile | null>(null);
  const [shareFile, setShareFile] = useState<DiskFile | null>(null);
  const [tagFile, setTagFile] = useState<DiskFile | null>(null);
  const [showPreview, setShowPreview] = useState(false);

  const { data: ancestors = [] } = useQuery({
    queryKey: ['folder-ancestors', folderId],
    queryFn: () => folderApi.getAncestors(folderId).then(r => (r || []).map(a => ({ id: a.id, name: a.folderName }))),
    enabled: folderId > 0,
  });

  const handleRefresh = () => {
    queryClient.invalidateQueries({ queryKey: ['folders', folderId] });
    queryClient.invalidateQueries({ queryKey: ['files', folderId] });
  };

  if (showPreview && previewFile) {
    return (
      <FilePreview
        file={previewFile}
        onClose={() => { setShowPreview(false); setPreviewFile(null); }}
      />
    );
  }

  return (
    <div>
      {folderId > 0 && <BreadcrumbNav items={ancestors} />}

      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Space>
          <FileUpload folderId={folderId} />
          <Button icon={<FolderAddOutlined />} onClick={() => setCreateFolderOpen(true)}>
            新建文件夹
          </Button>
          <Button icon={<ReloadOutlined />} onClick={handleRefresh}>
            刷新
          </Button>
        </Space>
      </div>

      <FileList
        folderId={folderId}
        onPreview={(file) => { setPreviewFile(file); setShowPreview(true); }}
        onVersionHistory={(file) => setVersionFile(file)}
        onShare={(file) => setShareFile(file)}
        onTag={(file) => setTagFile(file)}
      />

      <CreateFolderModal
        open={createFolderOpen}
        parentId={folderId}
        onClose={() => setCreateFolderOpen(false)}
      />

      <VersionHistory
        file={versionFile}
        open={!!versionFile}
        onClose={() => setVersionFile(null)}
      />

      <CreateShareModal
        file={shareFile}
        open={!!shareFile}
        onClose={() => setShareFile(null)}
      />

      <TagInput
        file={tagFile}
        open={!!tagFile}
        onClose={() => setTagFile(null)}
      />
    </div>
  );
}
