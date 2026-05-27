import { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, Space, message, Tag, Popconfirm, Alert } from 'antd';
import { CopyOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
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
  updatedAt: string;
}

const DELETE_WARNING = '该 API key 将立即被禁用。使用此密钥发出的 API 请求将被拒绝，这可能会导致仍然依赖它的任何系统崩溃。一旦删除，你将无法再查看或修改此 API key。';

export default function ApiKeyManager() {
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [createdKey, setCreatedKey] = useState<string>('');
  const [creating, setCreating] = useState(false);
  const [editModalOpen, setEditModalOpen] = useState(false);
  const [editingKey, setEditingKey] = useState<ApiKey | null>(null);
  const [renaming, setRenaming] = useState(false);
  const [form] = Form.useForm();
  const [editForm] = Form.useForm();

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
    setCreating(true);
    try {
      const res = await adminApi.createApiKey(values);
      setCreatedKey(res.data.key);
      form.resetFields();
      load();
    } catch (err: unknown) {
      message.error(err instanceof Error ? err.message : '创建失败');
    }
    setCreating(false);
  };

  const handleRename = async (values: { name: string }) => {
    if (!editingKey) return;
    setRenaming(true);
    try {
      await adminApi.renameApiKey(editingKey.id, values);
      message.success('已修改');
      setEditModalOpen(false);
      setEditingKey(null);
      load();
    } catch (err: unknown) {
      message.error(err instanceof Error ? err.message : '修改失败');
    }
    setRenaming(false);
  };

  const handleRevoke = async (id: number) => {
    try {
      await adminApi.revokeApiKey(id);
      message.success('已删除');
      load();
    } catch { message.error('删除失败'); }
  };

  const openEditModal = (record: ApiKey) => {
    setEditingKey(record);
    editForm.setFieldsValue({ name: record.keyName });
    setEditModalOpen(true);
  };

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      message.success('已复制到剪贴板');
    } catch {
      message.error('复制失败，请手动复制');
    }
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
        <Table.Column title="修改时间" dataIndex="updatedAt" />
        <Table.Column title="操作" width={100} render={(_: unknown, record: ApiKey) => (
          !record.isRevoked && !createdKey ? (
            <Space size={4}>
              <Button type="text" size="small" icon={<EditOutlined />} aria-label="编辑" onClick={() => openEditModal(record)} />
              <Popconfirm
                title={DELETE_WARNING}
                onConfirm={() => handleRevoke(record.id)}
                okText="删除"
                cancelText="取消"
                okButtonProps={{ danger: true }}
              >
                <Button type="text" size="small" danger icon={<DeleteOutlined />} aria-label="删除" />
              </Popconfirm>
            </Space>
          ) : <span style={{ color: '#999' }}>{record.isRevoked ? '已吊销' : '-'}</span>
        )} />
      </Table>

      {/* Create API Key Modal */}
      <Modal
        title="创建 API Key"
        open={modalOpen}
        onCancel={() => { setModalOpen(false); setCreatedKey(''); }}
        maskClosable={false}
        onOk={() => form.submit()}
        confirmLoading={creating}
        footer={createdKey ? (
          <Space>
            <Button onClick={() => { setModalOpen(false); setCreatedKey(''); }}>关闭</Button>
            <Button type="primary" icon={<CopyOutlined />} onClick={() => copyToClipboard(createdKey)}>复制</Button>
          </Space>
        ) : undefined}
      >
        {createdKey ? (
          <div>
            <Input.TextArea value={createdKey} rows={3} readOnly style={{ fontFamily: 'monospace' }} />
            <Alert
              style={{ marginTop: 12 }}
              type="warning"
              showIcon
              description="请将此 API key 保存在安全且易于访问的地方。出于安全原因，你将无法通过 API keys 管理界面再次查看它。如果你丢失了这个 key，将需要重新创建。"
            />
          </div>
        ) : (
          <Form form={form} onFinish={handleCreate} layout="vertical">
            <Form.Item name="name" label="Key 名称" rules={[{ required: true }]}>
              <Input placeholder="请输入Key 名称" />
            </Form.Item>
            <Form.Item name="department" label="部门（留空=全部）">
              <Input placeholder="请输入部门" />
            </Form.Item>
          </Form>
        )}
      </Modal>

      {/* Edit API Key Modal */}
      <Modal
        title="修改 API Key 名称"
        open={editModalOpen}
        onCancel={() => { setEditModalOpen(false); setEditingKey(null); }}
        onOk={() => editForm.submit()}
        confirmLoading={renaming}
      >
        <Form form={editForm} onFinish={handleRename} layout="vertical">
          <Form.Item name="name" label="Key 名称" rules={[{ required: true }]}>
            <Input placeholder="请输入Key 名称" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
