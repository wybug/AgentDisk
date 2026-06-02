import { Modal, Form, Input, message } from 'antd';
import { folderApi } from '@/api/folder';
import { useQueryClient } from '@tanstack/react-query';

interface Props {
  open: boolean;
  parentId: number;
  onClose: () => void;
}

export default function CreateFolderModal({ open, parentId, onClose }: Props) {
  const [form] = Form.useForm();
  const queryClient = useQueryClient();

  const handleOk = async () => {
    try {
      const values = await form.validateFields();
      await folderApi.create({ parentId, folderName: values.folderName });
      queryClient.invalidateQueries({ queryKey: ['folders', parentId] });
      queryClient.invalidateQueries({ queryKey: ['files', parentId] });
      form.resetFields();
      onClose();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg) {
        message.error(msg);
      }
    }
  };

  return (
    <Modal
      title="新建文件夹"
      open={open}
      onOk={handleOk}
      onCancel={() => { form.resetFields(); onClose(); }}
      okText="创建"
      cancelText="取消"
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="folderName"
          label="文件夹名称"
          rules={[{ required: true, message: '请输入文件夹名称' }]}
        >
          <Input placeholder="请输入文件夹名称" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
