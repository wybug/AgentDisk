import { Drawer, Table, Button, message, Popconfirm } from 'antd';
import { RollbackOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { versionApi } from '@/api/version';
import { formatFileSize, formatDate } from '@/utils/format';
import type { DiskFileVersion } from '@/api/types';
import type { DiskFile } from '@/api/types';

interface Props {
  file: DiskFile | null;
  open: boolean;
  onClose: () => void;
}

export default function VersionHistory({ file, open, onClose }: Props) {
  const queryClient = useQueryClient();

  const { data: versions = [], isLoading } = useQuery({
    queryKey: ['versions', file?.id],
    queryFn: () => versionApi.list(file!.id),
    enabled: !!file && open,
  });

  const handleRollback = async (fileId: number, version: number) => {
    try {
      await versionApi.rollback(fileId, version);
      message.success(`已回滚到 v${version}`);
      queryClient.invalidateQueries({ queryKey: ['files'] });
      queryClient.invalidateQueries({ queryKey: ['versions', fileId] });
    } catch (err: unknown) {
      message.error('回滚失败: ' + (err instanceof Error ? err.message : String(err)));
    }
  };

  return (
    <Drawer
      title={`版本历史 - ${file?.fileName || ''}`}
      open={open}
      onClose={onClose}
      width={600}
    >
      <Table
        dataSource={versions}
        loading={isLoading}
        rowKey="id"
        pagination={false}
        size="small"
        columns={[
          { title: '版本', dataIndex: 'version', width: 60, render: (v: number) => `v${v}` },
          { title: '大小', dataIndex: 'fileSize', width: 100, render: formatFileSize },
          { title: '快照来源', dataIndex: 'snapshotBy', width: 120 },
          { title: '创建时间', dataIndex: 'createdAt', width: 170, render: formatDate },
          {
            title: '操作',
            width: 80,
            render: (_: unknown, record: DiskFileVersion) => (
              <Popconfirm
                title={`确定回滚到 v${record.version}？`}
                onConfirm={() => handleRollback(record.fileId, record.version)}
              >
                <Button size="small" icon={<RollbackOutlined />}>回滚</Button>
              </Popconfirm>
            ),
          },
        ]}
      />
    </Drawer>
  );
}
