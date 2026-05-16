import { Dropdown, Modal, message } from 'antd';
import {
  DownloadOutlined,
  DeleteOutlined,
  ShareAltOutlined,
  EyeOutlined,
  HistoryOutlined,
  TagOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { fileApi } from '@/api/file';
import { getDownloadUrl } from '@/utils/format';
import { useQueryClient } from '@tanstack/react-query';
import type { DiskFile } from '@/api/types';

interface Props {
  file: DiskFile;
  folderId: number;
  onPreview?: () => void;
  onVersionHistory?: () => void;
  onShare?: () => void;
  onTag?: () => void;
}

export default function FileActions({ file, folderId, onPreview, onVersionHistory, onShare, onTag }: Props) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const handleDownload = async () => {
    try {
      const result = await fileApi.getDownloadToken(file.id);
      window.open(getDownloadUrl(result.downloadToken), '_blank');
    } catch (err: any) {
      message.error('获取下载链接失败: ' + err.message);
    }
  };

  const handleDelete = () => {
    Modal.confirm({
      title: `确定删除文件「${file.fileName}」？`,
      content: '文件将移至回收站',
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        await fileApi.delete(file.id);
        queryClient.invalidateQueries({ queryKey: ['files', folderId] });
        message.success('已移至回收站');
      },
    });
  };

  const items = [
    { key: 'preview', icon: <EyeOutlined />, label: '预览', onClick: onPreview },
    { key: 'download', icon: <DownloadOutlined />, label: '下载', onClick: handleDownload },
    { key: 'share', icon: <ShareAltOutlined />, label: '分享', onClick: onShare },
    { key: 'tag', icon: <TagOutlined />, label: '标签', onClick: onTag },
    { key: 'version', icon: <HistoryOutlined />, label: '版本历史', onClick: onVersionHistory },
    { type: 'divider' as const },
    { key: 'delete', icon: <DeleteOutlined />, label: '删除', danger: true, onClick: handleDelete },
  ].filter(Boolean);

  return <Dropdown menu={{ items }} trigger={['click']}><a>操作</a></Dropdown>;
}
