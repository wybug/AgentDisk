import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { Card, Input, Button, Result, Typography, Space } from 'antd';
import { LockOutlined, DownloadOutlined } from '@ant-design/icons';
import { shareApi } from '@/api/share';
import { fileApi } from '@/api/file';
import { getDownloadUrl } from '@/utils/format';
import type { DiskShare } from '@/api/types';

export default function ShareAccessPage() {
  const { code } = useParams<{ code: string }>();
  const [extractCode, setExtractCode] = useState('');
  const [shareInfo, setShareInfo] = useState<DiskShare | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [accessed, setAccessed] = useState(false);

  const handleAccess = async () => {
    if (!code) return;
    setLoading(true);
    setError('');
    try {
      const result = await shareApi.getPublic(code);
      setShareInfo(result);

      if (result.extractCode) {
        if (!extractCode) {
          setError('请输入提取码');
          setLoading(false);
          return;
        }
        await shareApi.accessPublic(code, extractCode);
      } else {
        await shareApi.accessPublic(code);
      }
      setAccessed(true);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '访问失败');
    } finally {
      setLoading(false);
    }
  };

  const handleDownload = async () => {
    if (!shareInfo) return;
    try {
      const tokenResult = await fileApi.getDownloadToken(shareInfo.resourceId);
      window.open(getDownloadUrl(tokenResult.downloadToken), '_blank');
    } catch (err: unknown) {
      setError('下载失败: ' + (err instanceof Error ? err.message : String(err)));
    }
  };

  if (accessed && shareInfo) {
    return (
      <div style={{ maxWidth: 600, margin: '80px auto', padding: '0 16px' }}>
        <Card>
          <Result
            status="success"
            title="分享验证成功"
            subTitle={`资源类型: ${shareInfo.resType} | ID: ${shareInfo.resourceId}`}
            extra={[
              <Button key="download" type="primary" icon={<DownloadOutlined />} onClick={handleDownload}>
                下载文件
              </Button>,
            ]}
          />
        </Card>
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 400, margin: '80px auto', padding: '0 16px' }}>
      <Card>
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <LockOutlined style={{ fontSize: 48, color: '#667eea' }} />
          <Typography.Title level={4} style={{ marginTop: 16 }}>
            访问分享
          </Typography.Title>
          <Typography.Text type="secondary">分享码: {code}</Typography.Text>
        </div>

        <Space direction="vertical" style={{ width: '100%' }}>
          <Input
            prefix={<LockOutlined />}
            placeholder="输入提取码"
            value={extractCode}
            onChange={(e) => setExtractCode(e.target.value)}
            onPressEnter={handleAccess}
          />
          {error && <Typography.Text type="danger">{error}</Typography.Text>}
          <Button
            type="primary"
            block
            loading={loading}
            onClick={handleAccess}
          >
            访问
          </Button>
        </Space>
      </Card>
    </div>
  );
}
