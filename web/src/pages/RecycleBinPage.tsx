import { Table, Button, Popconfirm, message, Space } from 'antd';
import { UndoOutlined, DeleteOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { recycleApi } from '@/api/recycle';
import { formatDate } from '@/utils/format';
import type { DiskRecycleBin } from '@/api/types';

export default function RecycleBinPage() {
  const queryClient = useQueryClient();

  const { data: items = [], isLoading } = useQuery({
    queryKey: ['recycle'],
    queryFn: () => recycleApi.list(),
  });

  const handleRestore = async (recycleId: number) => {
    try {
      await recycleApi.restore(recycleId);
      message.success('已恢复');
      queryClient.invalidateQueries({ queryKey: ['recycle'] });
      queryClient.invalidateQueries({ queryKey: ['files'] });
      queryClient.invalidateQueries({ queryKey: ['folders'] });
    } catch (err: unknown) {
      message.error('恢复失败: ' + (err instanceof Error ? err.message : String(err)));
    }
  };

  const handlePermanentDelete = async (recycleId: number) => {
    try {
      await recycleApi.deletePermanent(recycleId);
      message.success('已彻底删除');
      queryClient.invalidateQueries({ queryKey: ['recycle'] });
      queryClient.invalidateQueries({ queryKey: ['space'] });
    } catch (err: unknown) {
      message.error('删除失败: ' + (err instanceof Error ? err.message : String(err)));
    }
  };

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}>回收站</h2>
      <Table
        dataSource={items}
        loading={isLoading}
        rowKey="id"
        pagination={false}
        columns={[
          { title: '名称', dataIndex: 'resName' },
          { title: '类型', dataIndex: 'resType', width: 80 },
          { title: '原路径', dataIndex: 'originalPath' },
          { title: '删除来源', dataIndex: 'deletedBy', width: 100 },
          { title: '删除时间', dataIndex: 'createdAt', width: 170, render: formatDate },
          {
            title: '操作',
            width: 150,
            render: (_: unknown, record: DiskRecycleBin) => (
              <Space>
                <Button
                  size="small"
                  icon={<UndoOutlined />}
                  onClick={() => handleRestore(record.id)}
                >
                  恢复
                </Button>
                <Popconfirm
                  title="确定彻底删除？此操作不可恢复"
                  onConfirm={() => handlePermanentDelete(record.id)}
                >
                  <Button size="small" danger icon={<DeleteOutlined />}>
                    彻底删除
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />
    </div>
  );
}
