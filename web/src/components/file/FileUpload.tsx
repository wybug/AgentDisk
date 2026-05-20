import { Upload, Button, message } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import { fileApi } from '@/api/file';
import { useQueryClient } from '@tanstack/react-query';
import type { UploadProps } from 'antd';

interface Props {
  folderId: number;
}

export default function FileUpload({ folderId }: Props) {
  const queryClient = useQueryClient();

  const uploadProps: UploadProps = {
    name: 'file',
    multiple: true,
    showUploadList: false,
    customRequest: async (options) => {
      const { file, onSuccess, onError } = options;
      try {
        await fileApi.upload(file as File, folderId);
        onSuccess?.(null);
        queryClient.invalidateQueries({ queryKey: ['files', folderId] });
        queryClient.invalidateQueries({ queryKey: ['space'] });
        message.success(`${(file as File).name} 上传成功`);
      } catch (err: unknown) {
        onError?.(err as Error);
        message.error(`${(file as File).name} 上传失败: ${err instanceof Error ? err.message : String(err)}`);
      }
    },
  };

  return (
    <Upload {...uploadProps}>
      <Button type="primary" icon={<UploadOutlined />}>上传文件</Button>
    </Upload>
  );
}
