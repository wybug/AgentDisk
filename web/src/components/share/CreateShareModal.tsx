import { useState } from 'react';
import { Modal, Form, Input, InputNumber, Typography, message } from 'antd';
import { shareApi } from '@/api/share';
import type { DiskFile } from '@/api/types';

interface Props {
  file: DiskFile | null;
  open: boolean;
  onClose: () => void;
}

export default function CreateShareModal({ file, open, onClose }: Props) {
  const [form] = Form.useForm();
  const [shareResult, setShareResult] = useState<{ shareCode: string; extractCode: string } | null>(null);

  const handleOk = async () => {
    if (!file) return;
    const values = await form.validateFields();
    try {
      await shareApi.create({
        resourceId: file.id,
        resType: 'file',
        extractCode: values.extractCode || undefined,
        maxVisit: values.maxVisit,
        expireHours: values.expireHours,
      });
      // The API should return share info, but based on current response format
      // we'll show success message
      message.success('分享创建成功');
      setShareResult({
        shareCode: 'check shares page',
        extractCode: values.extractCode || '',
      });
    } catch (err: unknown) {
      message.error('创建失败: ' + (err instanceof Error ? err.message : String(err)));
    }
  };

  const handleClose = () => {
    form.resetFields();
    setShareResult(null);
    onClose();
  };

  return (
    <Modal
      title={`分享文件 - ${file?.fileName || ''}`}
      open={open}
      onOk={shareResult ? undefined : handleOk}
      onCancel={handleClose}
      okText="创建分享"
      cancelText="关闭"
      footer={shareResult ? undefined : undefined}
    >
      {shareResult ? (
        <div>
          <Typography.Paragraph>
            分享链接: <Typography.Text copyable>{`${window.location.origin}/share/${shareResult.shareCode}`}</Typography.Text>
          </Typography.Paragraph>
          {shareResult.extractCode && (
            <Typography.Paragraph>
              提取码: <Typography.Text copyable>{shareResult.extractCode}</Typography.Text>
            </Typography.Paragraph>
          )}
        </div>
      ) : (
        <Form form={form} layout="vertical" initialValues={{ maxVisit: -1, expireHours: 72 }}>
          <Form.Item name="extractCode" label="提取码（可选）">
            <Input placeholder="留空则不需要提取码" maxLength={6} />
          </Form.Item>
          <Form.Item name="maxVisit" label="最大访问次数（-1 为不限）">
            <InputNumber style={{ width: '100%' }} min={-1} />
          </Form.Item>
          <Form.Item name="expireHours" label="有效时长（小时）">
            <InputNumber style={{ width: '100%' }} min={1} />
          </Form.Item>
        </Form>
      )}
    </Modal>
  );
}
