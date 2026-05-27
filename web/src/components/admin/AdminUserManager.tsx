import { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, message, Popconfirm, Tag } from 'antd';
import { adminApi } from '@/api/admin';

interface AdminUser {
  username: string;
  role: string;
  displayName: string;
  isActive: boolean;
  createdBy: string;
}

export default function AdminUserManager() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();

  const load = async () => {
    setLoading(true);
    try {
      const res = await adminApi.listUsers();
      setUsers(res.data || []);
    } catch { message.error('加载失败'); }
    setLoading(false);
  };

  useEffect(() => { load(); }, []); // eslint-disable-line react-hooks/set-state-in-effect

  const handleCreate = async (values: { username: string; password: string; role?: string; displayName?: string }) => {
    try {
      await adminApi.createUser(values);
      message.success('创建成功');
      setModalOpen(false);
      form.resetFields();
      load();
    } catch (err: unknown) {
      message.error(err instanceof Error ? err.message : '创建失败');
    }
  };

  const handleDelete = async (username: string) => {
    try {
      await adminApi.deleteUser(username);
      message.success('删除成功');
      load();
    } catch { message.error('删除失败'); }
  };

  const handleResetPwd = async (username: string) => {
    const password = prompt('输入新密码（至少6位）');
    if (!password) return;
    try {
      await adminApi.changePassword(username, password);
      message.success('密码已更新');
    } catch { message.error('更新失败'); }
  };

  return (
    <>
      <div style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => setModalOpen(true)}>创建管理员</Button>
      </div>
      <Table dataSource={users} rowKey="username" loading={loading} pagination={false}>
        <Table.Column title="用户名" dataIndex="username" />
        <Table.Column title="角色" dataIndex="role" render={(v: string) => <Tag color={v === 'super_admin' ? 'gold' : 'blue'}>{v}</Tag>} />
        <Table.Column title="显示名" dataIndex="displayName" render={(v: string) => v || '-'} />
        <Table.Column title="状态" dataIndex="isActive" render={(v: boolean) => <Tag color={v ? 'green' : 'red'}>{v ? '启用' : '禁用'}</Tag>} />
        <Table.Column title="创建者" dataIndex="createdBy" />
        <Table.Column title="操作" render={(_: unknown, record: AdminUser) => (
          <>
            <Button type="link" onClick={() => handleResetPwd(record.username)}>改密码</Button>
            <Popconfirm title="确定删除？" onConfirm={() => handleDelete(record.username)}>
              <Button type="link" danger>删除</Button>
            </Popconfirm>
          </>
        )} />
      </Table>

      <Modal title="创建管理员" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => form.submit()}>
        <Form form={form} onFinish={handleCreate} layout="vertical">
          <Form.Item name="username" label="用户名" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true, min: 6 }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="role" label="角色" initialValue="admin">
            <Input placeholder="admin / super_admin" />
          </Form.Item>
          <Form.Item name="displayName" label="显示名">
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
