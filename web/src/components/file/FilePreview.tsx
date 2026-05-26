import { useEffect, useState } from 'react';
import { Spin, Image, Button, Typography, Space, Alert } from 'antd';
import { DownloadOutlined, ArrowLeftOutlined } from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { fileApi } from '@/api/file';
import { getDownloadUrl } from '@/utils/format';
import { classifyFile } from '@/utils/fileType';
import type { DiskFile, PreviewResult } from '@/api/types';

interface Props {
  file: DiskFile;
  onClose: () => void;
}

function getLanguage(fileName: string): string {
  const ext = fileName.lastIndexOf('.') !== -1 ? fileName.slice(fileName.lastIndexOf('.') + 1).toLowerCase() : '';
  const map: Record<string, string> = {
    js: 'javascript', jsx: 'jsx', ts: 'typescript', tsx: 'tsx',
    py: 'python', go: 'go', java: 'java', c: 'c', cpp: 'cpp',
    rs: 'rust', rb: 'ruby', php: 'php', swift: 'swift', kt: 'kotlin',
    sh: 'bash', css: 'css', html: 'html', xml: 'xml', sql: 'sql',
    json: 'json', yaml: 'yaml', yml: 'yaml', md: 'markdown',
  };
  return map[ext] || 'text';
}

export default function FilePreview({ file, onClose }: Props) {
  const [loading, setLoading] = useState(true);
  const [preview, setPreview] = useState<PreviewResult | null>(null);
  const [content, setContent] = useState<string>('');
  const [htmlPreviewUrl, setHtmlPreviewUrl] = useState<string | null>(null);
  const [htmlAllowScripts, setHtmlAllowScripts] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const result = await fileApi.preview(file.id);
        if (cancelled) return;
        setPreview(result);

        const category = result.fileType || classifyFile(file.fileName);
        if (category === 'html') {
          const tokenResult = await fileApi.getDownloadToken(file.id);
          if (!cancelled) setHtmlPreviewUrl(`/v1/disk/preview/${file.id}/html?t=${encodeURIComponent(tokenResult.downloadToken)}`);
        } else if (['markdown', 'code', 'text'].includes(category) && result.url) {
          const resp = await fetch(result.url);
          const text = await resp.text();
          if (!cancelled) setContent(text);
        }
      } catch {
        // error handled by interceptor
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [file.id, file.fileName]);

  const handleDownload = async () => {
    const result = await fileApi.getDownloadToken(file.id);
    window.open(getDownloadUrl(result.downloadToken), '_blank');
  };

  if (loading) return <Spin />;

  if (!preview) return <Typography.Text type="secondary">无法预览此文件</Typography.Text>;

  const category = preview.fileType || classifyFile(file.fileName);

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Space>
          <Button icon={<ArrowLeftOutlined />} onClick={onClose}>返回</Button>
          <Typography.Text strong>{file.fileName}</Typography.Text>
        </Space>
        <Button icon={<DownloadOutlined />} onClick={handleDownload}>下载</Button>
      </div>

      {category === 'image' && (
        <div style={{ textAlign: 'center' }}>
          <Image src={preview.url} alt={file.fileName} style={{ maxWidth: '100%' }} />
        </div>
      )}

      {category === 'markdown' && (
        <div style={{ padding: 24, background: '#fafafa', borderRadius: 8, overflow: 'auto', maxHeight: 600 }}>
          <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
        </div>
      )}

      {category === 'code' && (
        <SyntaxHighlighter
          language={getLanguage(file.fileName)}
          style={oneDark}
          showLineNumbers
          customStyle={{ maxHeight: 600, borderRadius: 8 }}
        >
          {content}
        </SyntaxHighlighter>
      )}

      {category === 'text' && (
        <pre style={{ padding: 16, background: '#f5f5f5', borderRadius: 8, maxHeight: 600, overflow: 'auto' }}>
          {content}
        </pre>
      )}

      {category === 'html' && htmlPreviewUrl && (
        <div>
          {!htmlAllowScripts && (
            <Alert
              message="JavaScript 已禁用"
              description="HTML 预览默认禁用 JavaScript 以确保安全。仅在信任此文件时可手动启用。"
              type="warning"
              showIcon
              action={
                <Button size="small" danger onClick={() => setHtmlAllowScripts(true)}>
                  启用脚本
                </Button>
              }
              style={{ marginBottom: 12 }}
            />
          )}
          <iframe
            src={htmlPreviewUrl}
            sandbox={htmlAllowScripts ? 'allow-scripts' : ''}
            style={{
              width: '100%',
              height: 600,
              border: '1px solid #d9d9d9',
              borderRadius: 8,
            }}
            title={`Preview of ${file.fileName}`}
            referrerPolicy="no-referrer"
          />
        </div>
      )}

      {category === 'binary' && (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Typography.Text type="secondary">此文件类型暂不支持预览</Typography.Text>
          <br />
          <Button icon={<DownloadOutlined />} onClick={handleDownload} style={{ marginTop: 16 }}>下载文件</Button>
        </div>
      )}
    </div>
  );
}
