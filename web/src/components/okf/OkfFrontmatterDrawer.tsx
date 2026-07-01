import { Drawer, Descriptions, Tag, Typography, Space, Button } from 'antd';
import { FileTextOutlined, WarningOutlined } from '@ant-design/icons';
import type { OkfNode } from '@/api/types';

interface Props {
  node: OkfNode | null;
  onClose: () => void;
  onPreview?: (fileId: number) => void;
}

// OkfFrontmatterDrawer shows the parsed frontmatter of a selected node in a
// right-side drawer. The data is read-only — the writer is API-Key-only and
// not exposed from the browser UI. The optional onPreview callback lets the
// host page open the existing /v1/disk/preview/:fileId viewer for the body.
export default function OkfFrontmatterDrawer({ node, onClose, onPreview }: Props) {
  const open = node !== null;
  return (
    <Drawer
      title={node?.title || '节点详情'}
      placement="right"
      width={480}
      open={open}
      onClose={onClose}
      destroyOnClose
    >
      {node && (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="type">
              <Tag color="blue">{node.type}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="relPath">
              <Typography.Text code style={{ wordBreak: 'break-all' }}>
                {node.relPath}
              </Typography.Text>
            </Descriptions.Item>
            {node.description && (
              <Descriptions.Item label="description">
                {node.description}
              </Descriptions.Item>
            )}
            {node.timestamp && (
              <Descriptions.Item label="timestamp">{node.timestamp}</Descriptions.Item>
            )}
            <Descriptions.Item label="nodeId">{node.nodeId}</Descriptions.Item>
            <Descriptions.Item label="fileId">{node.fileId}</Descriptions.Item>
            <Descriptions.Item label="hasBrokenLink">
              {node.hasBrokenLink ? (
                <Tag icon={<WarningOutlined />} color="error">
                  存在死链
                </Tag>
              ) : (
                <Tag color="success">无</Tag>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="contentHash">
              <Typography.Text code style={{ wordBreak: 'break-all', fontSize: 12 }}>
                {node.contentHash}
              </Typography.Text>
            </Descriptions.Item>
          </Descriptions>

          {node.tags.length > 0 && (
            <div>
              <Typography.Text type="secondary">tags</Typography.Text>
              <div style={{ marginTop: 4 }}>
                {node.tags.map((t) => (
                  <Tag key={t} color="geekblue" style={{ marginBottom: 4 }}>
                    {t}
                  </Tag>
                ))}
              </div>
            </div>
          )}

          {Object.keys(node.extra).length > 0 && (
            <div>
              <Typography.Text type="secondary">extra</Typography.Text>
              <pre
                style={{
                  marginTop: 4,
                  background: '#f5f5f5',
                  padding: 12,
                  borderRadius: 6,
                  fontSize: 12,
                  overflow: 'auto',
                }}
              >
                {JSON.stringify(node.extra, null, 2)}
              </pre>
            </div>
          )}

          {onPreview && (
            <Button
              icon={<FileTextOutlined />}
              block
              onClick={() => onPreview(node.fileId)}
            >
              预览 Markdown
            </Button>
          )}
        </Space>
      )}
    </Drawer>
  );
}
