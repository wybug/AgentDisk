import { useEffect, useState } from 'react';
import { Form, Modal, Select, Typography, App } from 'antd';
import { publicDirectoryApi } from '@/api/publicDirectory';
import { okfApi } from '@/api/okf';

interface PublicDir {
  id: number;
  scope: string;
  department: string;
  displayName: string;
  fixedPath: string;
}

interface Props {
  open: boolean;
  onClose: () => void;
  onRegistered: () => void;
}

// RegisterBundleModal lets the user pick a visible public directory and
// register it as an OKF bundle. The directory must already contain an
// index.md with an okf_version frontmatter field; otherwise the backend
// responds with 400 and we surface the message via App.message.
export default function RegisterBundleModal({ open, onClose, onRegistered }: Props) {
  const [dirs, setDirs] = useState<PublicDir[]>([]);
  const [publicDirectoryId, setPublicDirectoryId] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const { message } = App.useApp();

  useEffect(() => {
    if (!open) return;
    publicDirectoryApi
      .listVisible()
      .then((res) => setDirs(res.data || []))
      .catch(() => setDirs([]));
  }, [open]);

  const handleOk = async () => {
    if (publicDirectoryId == null) {
      message.warning('请选择一个公共目录');
      return;
    }
    setLoading(true);
    try {
      await okfApi.registerBundle(publicDirectoryId);
      message.success('Bundle 注册成功');
      setPublicDirectoryId(null);
      onRegistered();
      onClose();
    } catch (err: unknown) {
      const msg =
        err instanceof Error
          ? err.message
          : typeof err === 'object' && err && 'response' in err
            ? // axios error: response.data.message is the backend's explanation
              ((err as { response?: { data?: { message?: string } } }).response?.data?.message ?? '注册失败')
            : '注册失败';
      message.error(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title="注册 OKF Bundle"
      open={open}
      onOk={handleOk}
      onCancel={onClose}
      confirmLoading={loading}
      okText="注册"
      cancelText="取消"
      destroyOnClose
    >
      <Typography.Paragraph type="secondary">
        选择一个可见的公共目录注册为 OKF bundle。该目录的根必须已包含{' '}
        <Typography.Text code>index.md</Typography.Text> 且 frontmatter 中声明了{' '}
        <Typography.Text code>okf_version</Typography.Text>。
      </Typography.Paragraph>
      <Form layout="vertical">
        <Form.Item label="公共目录" required>
          <Select
            placeholder="选择公共目录"
            value={publicDirectoryId ?? undefined}
            onChange={(v) => setPublicDirectoryId(v)}
            options={dirs.map((d) => ({
              value: d.id,
              label: `${d.displayName} (${d.scope === 'global' ? '全局' : d.department || '部门'})`,
            }))}
            notFoundContent="无可见的公共目录"
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}
