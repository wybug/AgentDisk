import { useState } from 'react';
import { Modal, Form, Input, Select, message, Tag, Typography } from 'antd';
import { useQueryClient } from '@tanstack/react-query';
import { permissionApi } from '@/api/permission';
import { parseAgentConfig, AGENT_CONFIG_PLACEHOLDER } from '@/utils/permission';

const { Text } = Typography;

interface GrantPermResource {
  id: number;
  name: string;
  resType: 'file' | 'folder';
}

interface Props {
  resource: GrantPermResource | null;
  open: boolean;
  onClose: () => void;
}

export default function GrantPermissionModal({ resource, open, onClose }: Props) {
  const [form] = Form.useForm();
  const queryClient = useQueryClient();

  const handleOk = async () => {
    if (!resource) return;
    const values = await form.validateFields();
    const config = parseAgentConfig(values.agentConfig);
    if (!config) {
      message.error('Agent 配置格式错误');
      return;
    }
    try {
      await permissionApi.grant({
        ...config,
        resourceId: resource.id,
        resType: resource.resType,
        permission: values.permission,
      });
      message.success('权限已授予');
      queryClient.invalidateQueries({ queryKey: ['permissions'] });
      form.resetFields();
      onClose();
    } catch (err: unknown) {
      message.error('授予失败: ' + (err instanceof Error ? err.message : String(err)));
    }
  };

  const handleClose = () => {
    form.resetFields();
    onClose();
  };

  return (
    <Modal
      title={`授予权限 - ${resource?.name || ''}`}
      open={open}
      onOk={handleOk}
      onCancel={handleClose}
      okText="授予"
      cancelText="取消"
    >
      {resource && (
        <div style={{ marginBottom: 16, padding: '8px 12px', background: '#f5f5f5', borderRadius: 6 }}>
          <Tag color={resource.resType === 'folder' ? 'gold' : 'blue'}>
            {resource.resType === 'folder' ? '文件夹' : '文件'}
          </Tag>
          <Text strong>{resource.name}</Text>
          <Text type="secondary" style={{ marginLeft: 8 }}>ID: {resource.id}</Text>
        </div>
      )}
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
  );
}
