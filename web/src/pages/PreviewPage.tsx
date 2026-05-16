import { useParams, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Spin, Button, Result } from 'antd';
import { ArrowLeftOutlined } from '@ant-design/icons';
import { fileApi } from '@/api/file';
import FilePreview from '@/components/file/FilePreview';

export default function PreviewPage() {
  const { fileId } = useParams<{ fileId: string }>();
  const navigate = useNavigate();
  const id = fileId ? parseInt(fileId, 10) : 0;

  const { data: fileData, isLoading, error } = useQuery({
    queryKey: ['file', id],
    queryFn: () => fileApi.get(id),
    enabled: !!id,
  });

  if (isLoading) return <Spin size="large" style={{ display: 'block', margin: '80px auto' }} />;
  if (error || !fileData) {
    return (
      <Result
        status="404"
        title="文件未找到"
        extra={<Button onClick={() => navigate('/explorer')}>返回文件列表</Button>}
      />
    );
  }

  return (
    <FilePreview file={fileData.file} onClose={() => navigate(-1)} />
  );
}
