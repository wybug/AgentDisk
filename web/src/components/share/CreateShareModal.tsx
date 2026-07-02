import { useState } from 'react';
import { Modal, Form, Input, InputNumber, Typography, message } from 'antd';
import { shareApi } from '@/api/share';

export type ShareResType = 'file' | 'folder' | 'bundle';

interface Props {
  resource: { id: number; name: string } | null;
  resType: ShareResType;
  open: boolean;
  onClose: () => void;
}

// CreateShareModal is shared by the file/folder explorer and the OKF bundle
// detail page. resource is {id, name}; resType picks the backend switch arm.
// The result stage shows a copyable share link + extract code (if any).
export default function CreateShareModal({ resource, resType, open, onClose }: Props) {
  const [form] = Form.useForm();
  const [shareResult, setShareResult] = useState<{ shareCode: string; extractCode: string } | null>(null);

  const handleOk = async () => {
    if (!resource) return;
    const values = await form.validateFields();
    try {
      const share = await shareApi.create({
        resourceId: resource.id,
        resType,
        extractCode: values.extractCode || undefined,
        maxVisit: values.maxVisit,
        expireHours: values.expireHours,
      });
      message.success('分享创建成功');
      setShareResult({
        shareCode: share.shareCode,
        extractCode: share.extractCode || values.extractCode || '',
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

  const resTypeLabel = resType === 'bundle' ? 'Bundle' : resType === 'folder' ? '文件夹' : '文件';

  return (
    <Modal
      title={`分享${resTypeLabel} - ${resource?.name || ''}`}
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
