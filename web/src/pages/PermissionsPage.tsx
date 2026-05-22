import { useState } from 'react';
import { Table, Button, Modal, Form, Input, Select, Popconfirm, message, Tag, Typography } from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { permissionApi } from '@/api/permission';
import { parseAgentConfig, formatAgentTarget, formatResourceTarget, getAuthType, GLOB_HELP_TEXT, AGENT_CONFIG_PLACEHOLDER, PATH_PLACEHOLDER } from '@/utils/permission';
import type { DiskPermission } from '@/api/types';

const { Text } = Typography;

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
    const config = parseAgentConfig(values.agentConfig);
    if (!config) {
      message.error('Agent 配置格式错误，需要有效的 JSON 且包含 agentId 或 agentGroupId');
      return;
    }
    if (!values.resourcePath.startsWith('/')) {
      message.error('资源路径必须以 / 开头');
      return;
    }
    try {
      await permissionApi.grant({
        ...config,
        resourcePath: values.resourcePath,
        permission: values.permission,
      });
      message.success('权限已授予');
      queryClient.invalidateQueries({ queryKey: ['permissions'] });
      form.resetFields();
      setModalOpen(false);
    } catch (err: unknown) {
      message.error('授予失败: ' + (err instanceof Error ? err.message : String(err)));
    }
  };

  const handleRevoke = async (record: DiskPermission) => {
    try {
      await permissionApi.revoke({
        agentId: record.agentId || undefined,
        agentGroupId: record.agentGroupId || undefined,
        resourceId: record.resourceId || undefined,
        resType: record.resType || undefined,
        resourcePath: record.resourcePath || undefined,
      });
      message.success('权限已撤销');
      queryClient.invalidateQueries({ queryKey: ['permissions'] });
    } catch (err: unknown) {
      message.error('撤销失败: ' + (err instanceof Error ? err.message : String(err)));
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
          路径授权
        </Button>
      </div>

      <Table
        dataSource={permissions}
        loading={isLoading}
        rowKey="id"
        pagination={false}
        columns={[
          {
            title: '授权目标',
            width: 200,
            render: (_: unknown, record: DiskPermission) => <Text copyable={!!record.agentId}>{formatAgentTarget(record)}</Text>,
          },
          {
            title: '类型',
            width: 80,
            render: (_: unknown, record: DiskPermission) => (
              <Tag color={getAuthType(record) === 'path' ? 'purple' : 'cyan'}>
                {getAuthType(record) === 'path' ? '路径' : '资源ID'}
              </Tag>
            ),
          },
          {
            title: '资源',
            render: (_: unknown, record: DiskPermission) => (
              <Text copyable={getAuthType(record) === 'path'}>{formatResourceTarget(record)}</Text>
            ),
          },
          {
            title: '权限',
            dataIndex: 'permission',
            width: 80,
            render: (p: string) => <Tag color={permColors[p]}>{p}</Tag>,
          },
          {
            title: '操作',
            width: 80,
            render: (_: unknown, record: DiskPermission) => (
              <Popconfirm title="确定撤销此权限？" onConfirm={() => handleRevoke(record)}>
                <Button size="small" danger icon={<DeleteOutlined />}>撤销</Button>
              </Popconfirm>
            ),
          },
        ]}
      />

      <Modal
        title="路径授权"
        open={modalOpen}
        onOk={handleGrant}
        onCancel={() => { form.resetFields(); setModalOpen(false); }}
        okText="授予"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item name="agentConfig" label="Agent 配置" rules={[{ required: true, message: '请输入 Agent 配置' }]}>
            <Input.TextArea
              placeholder={AGENT_CONFIG_PLACEHOLDER}
              rows={3}
            />
          </Form.Item>
          <Text type="secondary" style={{ display: 'block', marginTop: -16, marginBottom: 16, fontSize: 12 }}>
            JSON 格式，至少包含 agentId 或 agentGroupId
          </Text>
          <Form.Item name="resourcePath" label="资源路径" rules={[{ required: true, message: '请输入资源路径' }]}>
            <Input placeholder={PATH_PLACEHOLDER} />
          </Form.Item>
          <Text type="secondary" style={{ display: 'block', marginTop: -16, marginBottom: 16, fontSize: 12 }}>
            {GLOB_HELP_TEXT}
          </Text>
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
