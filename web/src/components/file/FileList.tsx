import { Table, Tag, Button, Space, message } from 'antd';
import {
  FolderOutlined,
  FileOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { folderApi } from '@/api/folder';
import { fileApi } from '@/api/file';
import { formatFileSize, formatDate } from '@/utils/format';
import { getFileIcon } from '@/utils/fileType';
import type { DiskFile, DiskFolder } from '@/api/types';
import FileActions from './FileActions';
import * as AntdIcons from '@ant-design/icons';

interface Props {
  folderId: number;
  onPreview: (file: DiskFile) => void;
  onVersionHistory: (file: DiskFile) => void;
  onShare: (file: DiskFile) => void;
  onTag: (file: DiskFile) => void;
}

function IconFor(name: string) {
  const iconName = getFileIcon(name);
  const IconComponent = (AntdIcons as any)[iconName + 'Outlined'] || FileOutlined;
  return <IconComponent />;
}

export default function FileList({ folderId, onPreview, onVersionHistory, onShare, onTag }: Props) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const { data: folders = [] } = useQuery({
    queryKey: ['folders', folderId],
    queryFn: () => folderApi.list(folderId),
  });

  const { data: files = [], isLoading } = useQuery({
    queryKey: ['files', folderId],
    queryFn: () => fileApi.list(folderId),
  });

  const handleDeleteFolder = (folder: DiskFolder) => {
    AntdIcons.DeleteOutlined;
    import('antd').then(({ Modal, message }) => {
      Modal.confirm({
        title: `确定删除文件夹「${folder.folderName}」？`,
        content: '文件夹及其内容将移至回收站',
        okText: '删除',
        okType: 'danger',
        cancelText: '取消',
        onOk: async () => {
          await folderApi.delete(folder.id);
          queryClient.invalidateQueries({ queryKey: ['folders', folderId] });
          message.success('已删除');
        },
      });
    });
  };

  const folderRows = folders.map((f) => ({
    key: `folder-${f.id}`,
    id: f.id,
    name: f.folderName,
    type: 'folder' as const,
    size: '-',
    fileType: '文件夹',
    version: '-',
    tags: null,
    updatedAt: f.updatedAt,
    rawData: f,
  }));

  const fileRows = files.map((f) => ({
    key: `file-${f.id}`,
    id: f.id,
    name: f.fileName,
    type: 'file' as const,
    size: formatFileSize(f.fileSize),
    fileType: f.fileType || '-',
    version: `v${f.version}`,
    tags: f.tags,
    updatedAt: f.updatedAt,
    rawData: f,
  }));

  const dataSource = [...folderRows, ...fileRows];

  return (
    <Table
      dataSource={dataSource}
      loading={isLoading}
      pagination={false}
      size="middle"
      columns={[
        {
          title: '名称',
          dataIndex: 'name',
          render: (name: string, record: any) => (
            <a
              onClick={() => {
                if (record.type === 'folder') {
                  navigate(`/explorer/${record.id}`);
                } else {
                  onPreview(record.rawData);
                }
              }}
              style={{ display: 'flex', alignItems: 'center', gap: 8 }}
            >
              {record.type === 'folder' ? <FolderOutlined style={{ color: '#faad14' }} /> : IconFor(name)}
              {name}
            </a>
          ),
        },
        { title: '大小', dataIndex: 'size', width: 100 },
        { title: '类型', dataIndex: 'fileType', width: 100 },
        { title: '版本', dataIndex: 'version', width: 60 },
        {
          title: '标签',
          dataIndex: 'tags',
          width: 150,
          render: (tags: string | null) => {
            if (!tags) return '-';
            return tags.split(',').map((t) => <Tag key={t}>{t}</Tag>);
          },
        },
        { title: '修改时间', dataIndex: 'updatedAt', width: 170, render: formatDate },
        {
          title: '操作',
          width: 80,
          render: (_: any, record: any) => {
            if (record.type === 'folder') {
              return (
                <Button
                  type="link"
                  danger
                  size="small"
                  onClick={() => handleDeleteFolder(record.rawData)}
                >
                  删除
                </Button>
              );
            }
            return (
              <FileActions
                file={record.rawData}
                folderId={folderId}
                onPreview={() => onPreview(record.rawData)}
                onVersionHistory={() => onVersionHistory(record.rawData)}
                onShare={() => onShare(record.rawData)}
                onTag={() => onTag(record.rawData)}
              />
            );
          },
        },
      ]}
    />
  );
}
