import { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, Select, Switch, message, Tag, Popconfirm } from 'antd';
import { adminApi } from '@/api/admin';

interface PublicDir {
  id: number;
  scope: string;
  department: string;
  displayName: string;
  fixedPath: string;
  isActive: boolean;
  createdBy: string;
}

export default function PublicDirManager() {
  const [dirs, setDirs] = useState<PublicDir[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();

  const load = async () => {
    setLoading(true);
    try {
      const res = await adminApi.listPublicDirectories();
      setDirs(res.data || []);
    } catch { message.error('加载失败'); }
    setLoading(false);
  };

  useEffect(() => { load(); }, []); // eslint-disable-line react-hooks/set-state-in-effect

  const handleCreate = async (values: { displayName: string; scope: string; department?: string }) => {
    try {
      await adminApi.createPublicDirectory(values);
      message.success('创建成功');
      setModalOpen(false);
      form.resetFields();
      load();
    } catch (err: unknown) {
      message.error(err instanceof Error ? err.message : '创建失败');
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await adminApi.deletePublicDirectory(id);
      message.success('删除成功');
      load();
    } catch { message.error('删除失败'); }
  };

  const handleToggle = async (record: PublicDir) => {
    try {
      await adminApi.updatePublicDirectory(record.id, { isActive: !record.isActive });
      load();
    } catch { message.error('更新失败'); }
  };

  return (
    <>
      <div style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => setModalOpen(true)}>创建公共目录</Button>
      </div>
      <Table dataSource={dirs} rowKey="id" loading={loading} pagination={false}>
        <Table.Column title="名称" dataIndex="displayName" />
        <Table.Column title="类型" dataIndex="scope" render={(v: string) => <Tag color={v === 'global' ? 'blue' : 'green'}>{v === 'global' ? '全局' : '部门'}</Tag>} />
        <Table.Column title="部门" dataIndex="department" render={(v: string) => v || '-'} />
        <Table.Column title="固定路径" dataIndex="fixedPath" />
        <Table.Column title="状态" dataIndex="isActive" render={(v: boolean, record: PublicDir) => <Switch checked={v} onChange={() => handleToggle(record)} />} />
        <Table.Column title="操作" render={(_: unknown, record: PublicDir) => (
          <Popconfirm title="确定删除？" onConfirm={() => handleDelete(record.id)}>
            <Button type="link" danger>删除</Button>
          </Popconfirm>
        )} />
      </Table>

      <Modal title="创建公共目录" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => form.submit()}>
        <Form form={form} onFinish={handleCreate} layout="vertical">
          <Form.Item name="displayName" label="目录名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="scope" label="类型" rules={[{ required: true }]} initialValue="global">
            <Select options={[{ value: 'global', label: '全局公共' }, { value: 'department', label: '部门公共' }]} />
          </Form.Item>
          <Form.Item name="department" label="部门标识">
            <Input placeholder="部门类型时必填" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
