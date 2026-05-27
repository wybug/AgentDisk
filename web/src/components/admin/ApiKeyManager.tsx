import { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, Space, message, Tag, Typography, Popconfirm } from 'antd';
import { CopyOutlined } from '@ant-design/icons';
import { adminApi } from '@/api/admin';

interface ApiKey {
  id: number;
  keyName: string;
  keyPrefix: string;
  scope: string;
  department: string;
  isRevoked: boolean;
  createdBy: string;
  createdAt: string;
}

export default function ApiKeyManager() {
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [createdKey, setCreatedKey] = useState<string>('');
  const [form] = Form.useForm();

  const load = async () => {
    setLoading(true);
    try {
      const res = await adminApi.listApiKeys();
      setKeys(res.data || []);
    } catch { message.error('加载失败'); }
    setLoading(false);
  };

  useEffect(() => { load(); }, []); // eslint-disable-line react-hooks/set-state-in-effect

  const handleCreate = async (values: { name: string; department?: string }) => {
    try {
      const res = await adminApi.createApiKey(values);
      setCreatedKey(res.data.key);
      form.resetFields();
      load();
    } catch (err: unknown) {
      message.error(err instanceof Error ? err.message : '创建失败');
    }
  };

  const handleRevoke = async (id: number) => {
    try {
      await adminApi.revokeApiKey(id);
      message.success('已吊销');
      load();
    } catch { message.error('吊销失败'); }
  };

  const copyKey = () => {
    navigator.clipboard.writeText(createdKey);
    message.success('已复制');
  };

  return (
    <>
      <div style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => { setCreatedKey(''); setModalOpen(true); }}>创建 API Key</Button>
      </div>
      <Table dataSource={keys} rowKey="id" loading={loading} pagination={false}>
        <Table.Column title="名称" dataIndex="keyName" />
        <Table.Column title="前缀" dataIndex="keyPrefix" render={(v: string) => <code>{v}...</code>} />
        <Table.Column title="部门" dataIndex="department" render={(v: string) => v || '全部'} />
        <Table.Column title="状态" dataIndex="isRevoked" render={(v: boolean) => <Tag color={v ? 'red' : 'green'}>{v ? '已吊销' : '有效'}</Tag>} />
        <Table.Column title="创建时间" dataIndex="createdAt" />
        <Table.Column title="操作" render={(_: unknown, record: ApiKey) => (
          !record.isRevoked ? (
            <Popconfirm title="确定吊销？" onConfirm={() => handleRevoke(record.id)}>
              <Button type="link" danger>吊销</Button>
            </Popconfirm>
          ) : <span style={{ color: '#999' }}>已吊销</span>
        )} />
      </Table>

      <Modal
        title="创建 API Key"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        footer={createdKey ? <Button onClick={() => setModalOpen(false)}>关闭</Button> : undefined}
      >
        {createdKey ? (
          <div>
            <Typography.Text type="warning">请立即复制，此 Key 仅显示一次：</Typography.Text>
            <Space style={{ marginTop: 8 }}>
              <Input.TextArea value={createdKey} rows={2} readOnly style={{ fontFamily: 'monospace' }} />
              <Button icon={<CopyOutlined />} onClick={copyKey}>复制</Button>
            </Space>
          </div>
        ) : (
          <Form form={form} onFinish={handleCreate} layout="vertical">
            <Form.Item name="name" label="Key 名称" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
            <Form.Item name="department" label="部门（留空=全部）">
              <Input />
            </Form.Item>
          </Form>
        )}
      </Modal>
    </>
  );
}
