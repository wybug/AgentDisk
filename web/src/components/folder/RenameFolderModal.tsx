import { Modal, Form, Input } from 'antd';
import { folderApi } from '@/api/folder';
import { useQueryClient } from '@tanstack/react-query';
import type { DiskFolder } from '@/api/types';

interface Props {
  folder: DiskFolder | null;
  open: boolean;
  onClose: () => void;
}

export default function RenameFolderModal({ folder, open, onClose }: Props) {
  const [form] = Form.useForm();
  const queryClient = useQueryClient();

  const handleOk = async () => {
    if (!folder) return;
    const values = await form.validateFields();
    await folderApi.rename(folder.id, values.folderName);
    queryClient.invalidateQueries({ queryKey: ['folders'] });
    form.resetFields();
    onClose();
  };

  return (
    <Modal
      title="重命名文件夹"
      open={open}
      onOk={handleOk}
      onCancel={() => { form.resetFields(); onClose(); }}
      okText="确定"
      cancelText="取消"
      afterOpenChange={(visible) => {
        if (visible && folder) {
          form.setFieldsValue({ folderName: folder.folderName });
        }
      }}
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="folderName"
          label="文件夹名称"
          rules={[{ required: true, message: '请输入文件夹名称' }]}
        >
          <Input placeholder="请输入新名称" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
