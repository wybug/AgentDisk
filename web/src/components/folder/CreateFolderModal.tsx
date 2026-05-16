import { Modal, Form, Input, InputNumber } from 'antd';
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
    const values = await form.validateFields();
    await folderApi.create({ parentId: values.parentId ?? parentId, folderName: values.folderName });
    queryClient.invalidateQueries({ queryKey: ['folders', parentId] });
    queryClient.invalidateQueries({ queryKey: ['files', parentId] });
    form.resetFields();
    onClose();
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
      <Form form={form} layout="vertical" initialValues={{ parentId }}>
        <Form.Item name="parentId" label="父文件夹 ID">
          <InputNumber style={{ width: '100%' }} min={0} />
        </Form.Item>
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
