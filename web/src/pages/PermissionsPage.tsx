import { useState } from 'react';
import { Table, Button, Modal, Form, Input, Select, Popconfirm, message, Space, Tag } from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { permissionApi } from '@/api/permission';

export default function PermissionsPage() {
  const queryClient = useQueryClient();
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();

  const { data: permissions = [], isLoading } = useQuery({
    queryKey: ['permissions'],
    queryFn: () => permissionApi.list(),
  });

  const handleGrant = async () => {
    const values = await form.validateFields();
    try {
      await permissionApi.grant(values);
      message.success('权限已授予');
      queryClient.invalidateQueries({ queryKey: ['permissions'] });
      form.resetFields();
      setModalOpen(false);
    } catch (err: any) {
      message.error('授予失败: ' + err.message);
    }
  };

  const handleRevoke = async (record: any) => {
    try {
      await permissionApi.revoke({
        agentId: record.agentId,
        resourceId: record.resourceId,
        resType: record.resType,
      });
      message.success('权限已撤销');
      queryClient.invalidateQueries({ queryKey: ['permissions'] });
    } catch (err: any) {
      message.error('撤销失败: ' + err.message);
    }
  };

  const permColors: Record<string, string> = {
    owner: 'red',
    read: 'blue',
    write: 'green',
    delete: 'orange',
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2 style={{ margin: 0 }}>权限管理</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
          授予权限
        </Button>
      </div>

      <Table
        dataSource={permissions}
        loading={isLoading}
        rowKey="id"
        pagination={false}
        columns={[
          { title: 'Agent ID', dataIndex: 'agentId' },
          { title: '资源 ID', dataIndex: 'resourceId', width: 80 },
          { title: '资源类型', dataIndex: 'resType', width: 100, render: (t: string) => <Tag>{t}</Tag> },
          {
            title: '权限',
            dataIndex: 'permission',
            width: 80,
            render: (p: string) => <Tag color={permColors[p]}>{p}</Tag>,
          },
          {
            title: '操作',
            width: 80,
            render: (_: any, record: any) => (
              <Popconfirm title="确定撤销此权限？" onConfirm={() => handleRevoke(record)}>
                <Button size="small" danger icon={<DeleteOutlined />}>撤销</Button>
              </Popconfirm>
            ),
          },
        ]}
      />

      <Modal
        title="授予权限"
        open={modalOpen}
        onOk={handleGrant}
        onCancel={() => { form.resetFields(); setModalOpen(false); }}
        okText="授予"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item name="agentId" label="Agent ID" rules={[{ required: true }]}>
            <Input placeholder="输入 Agent ID" />
          </Form.Item>
          <Form.Item name="resourceId" label="资源 ID" rules={[{ required: true }]}>
            <Input type="number" placeholder="输入资源 ID" />
          </Form.Item>
          <Form.Item name="resType" label="资源类型" rules={[{ required: true }]}>
            <Select options={[{ value: 'file', label: '文件' }, { value: 'folder', label: '文件夹' }]} />
          </Form.Item>
          <Form.Item name="permission" label="权限级别" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'owner', label: 'Owner (所有者)' },
                { value: 'read', label: 'Read (只读)' },
                { value: 'write', label: 'Write (读写)' },
                { value: 'delete', label: 'Delete (删除)' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
