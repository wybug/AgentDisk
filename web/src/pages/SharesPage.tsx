import { Table, Button, Popconfirm, message, Space, Tag } from 'antd';
import { DeleteOutlined, CopyOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { shareApi } from '@/api/share';
import { formatDate } from '@/utils/format';

export default function SharesPage() {
  const queryClient = useQueryClient();

  const { data: shares = [], isLoading } = useQuery({
    queryKey: ['shares'],
    queryFn: () => shareApi.list(),
  });

  const handleRevoke = async (shareId: number) => {
    try {
      await shareApi.revoke(shareId);
      message.success('已撤销分享');
      queryClient.invalidateQueries({ queryKey: ['shares'] });
    } catch (err: any) {
      message.error('撤销失败: ' + err.message);
    }
  };

  const copyLink = (code: string) => {
    navigator.clipboard.writeText(`${window.location.origin}/share/${code}`);
    message.success('链接已复制');
  };

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}>我的分享</h2>
      <Table
        dataSource={shares}
        loading={isLoading}
        rowKey="id"
        pagination={false}
        columns={[
          {
            title: '资源类型',
            dataIndex: 'resType',
            width: 100,
            render: (t: string) => <Tag>{t}</Tag>,
          },
          { title: '资源 ID', dataIndex: 'resourceId', width: 80 },
          {
            title: '分享码',
            dataIndex: 'shareCode',
            render: (code: string) => (
              <Space>
                <code>{code}</code>
                <Button type="link" size="small" icon={<CopyOutlined />} onClick={() => copyLink(code)} />
              </Space>
            ),
          },
          {
            title: '提取码',
            dataIndex: 'extractCode',
            width: 80,
            render: (code: string) => code || '-',
          },
          {
            title: '访问',
            width: 100,
            render: (_: any, record: any) => `${record.visitCount}/${record.maxVisit === -1 ? '∞' : record.maxVisit}`,
          },
          {
            title: '有效期',
            dataIndex: 'expireAt',
            width: 170,
            render: (d: string) => formatDate(d),
          },
          {
            title: '状态',
            dataIndex: 'isActive',
            width: 80,
            render: (active: boolean) => (
              <Tag color={active ? 'green' : 'red'}>{active ? '有效' : '已失效'}</Tag>
            ),
          },
          {
            title: '操作',
            width: 80,
            render: (_: any, record: any) => (
              <Popconfirm
                title="确定撤销此分享？"
                onConfirm={() => handleRevoke(record.id)}
              >
                <Button size="small" danger icon={<DeleteOutlined />}>撤销</Button>
              </Popconfirm>
            ),
          },
        ]}
      />
    </div>
  );
}
