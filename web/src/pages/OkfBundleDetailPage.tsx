import { useState } from 'react';
import { Breadcrumb, Button, Card, Space, Spin, Tabs, Tag, Typography, App } from 'antd';
import {
  ArrowLeftOutlined,
  FileSearchOutlined,
  ReloadOutlined,
  ScissorOutlined,
} from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { okfApi } from '@/api/okf';
import OkfNodeList from '@/components/okf/OkfNodeList';
import OkfGraphView from '@/components/okf/OkfGraphView';
import OkfStatsPanel from '@/components/okf/OkfStatsPanel';
import OkfBrokenLinksPanel from '@/components/okf/OkfBrokenLinksPanel';
import OkfFrontmatterDrawer from '@/components/okf/OkfFrontmatterDrawer';
import type { OkfNode } from '@/api/types';

// OkfBundleDetailPage renders the bundle header (title, counts, maintenance
// actions) and a 4-tab body: 节点 / 图谱 / 统计 / 死链. Tabs are mounted on
// demand (AntD default) so the graph only initializes Cytoscape when the
// user actually opens it.
export default function OkfBundleDetailPage() {
  const { bundleId } = useParams<{ bundleId: string }>();
  const id = Number(bundleId);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { message, modal } = App.useApp();
  const [activeTab, setActiveTab] = useState('nodes');
  const [selectedNode, setSelectedNode] = useState<OkfNode | null>(null);

  const { data: bundle, isLoading } = useQuery({
    queryKey: ['okf-bundle', id],
    queryFn: () => okfApi.getBundle(id),
    enabled: Number.isFinite(id) && id > 0,
  });

  const handleRefreshBundle = async () => {
    try {
      await okfApi.refreshBundle(id);
      message.success('Bundle 已刷新');
      queryClient.invalidateQueries({ queryKey: ['okf-bundle', id] });
    } catch {
      message.error('刷新失败');
    }
  };

  const handleRegenerateIndex = () => {
    modal.confirm({
      title: '重建索引',
      content: '将根据当前节点集合重新生成根 index.md，确认继续？',
      okText: '确认',
      cancelText: '取消',
      onOk: async () => {
        try {
          const res = await okfApi.regenerateIndex(id);
          message.success(`索引已重建（版本 ${res.indexVersion}）`);
        } catch {
          message.error('重建失败');
        }
      },
    });
  };

  const handleUnregister = () => {
    modal.confirm({
      title: '注销 Bundle',
      content: '将移除 bundle 注册和节点索引，底层 public directory 与文件保留。继续？',
      okText: '注销',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await okfApi.unregisterBundle(id);
          message.success('Bundle 已注销');
          queryClient.invalidateQueries({ queryKey: ['okf-bundles'] });
          navigate('/okf');
        } catch {
          message.error('注销失败');
        }
      },
    });
  };

  const handlePreview = (fileId: number) => {
    navigate(`/preview/${fileId}`);
  };

  if (isLoading) return <Spin style={{ display: 'block', margin: '80px auto' }} />;

  if (!bundle) {
    return <Typography.Text type="secondary">Bundle 不存在或不可见</Typography.Text>;
  }

  return (
    <>
      <Breadcrumb
        items={[
          { title: <a onClick={() => navigate('/okf')}><ArrowLeftOutlined /> OKF 知识库</a> },
          { title: bundle.title || `Bundle #${bundle.bundleId}` },
        ]}
        style={{ marginBottom: 16 }}
      />

      <Card
        title={
          <Space>
            <Typography.Text strong>{bundle.title || `Bundle #${bundle.bundleId}`}</Typography.Text>
            <Tag color={bundle.status === 'active' ? 'green' : 'default'}>{bundle.status}</Tag>
            <Tag color="blue">v{bundle.okfVersion}</Tag>
            <Tag>{bundle.nodeCount} 节点</Tag>
            <Tag>{bundle.edgeCount} 边</Tag>
          </Space>
        }
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={handleRefreshBundle}>
              刷新 Bundle
            </Button>
            <Button icon={<FileSearchOutlined />} onClick={handleRegenerateIndex}>
              重建索引
            </Button>
            <Button danger icon={<ScissorOutlined />} onClick={handleUnregister}>
              注销
            </Button>
          </Space>
        }
      >
        {bundle.description && (
          <Typography.Paragraph type="secondary">
            {bundle.description}
          </Typography.Paragraph>
        )}
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          publicDirectoryId: {bundle.publicDirectoryId} · 更新于{' '}
          {new Date(bundle.updatedAt).toLocaleString()}
        </Typography.Text>
      </Card>

      <Card style={{ marginTop: 16 }}>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'nodes',
              label: '节点',
              children: <OkfNodeList bundleId={id} onNodeClick={setSelectedNode} />,
            },
            {
              key: 'graph',
              label: '图谱',
              children: (
                <OkfGraphView bundleId={id} onNodeDoubleClick={setSelectedNode} />
              ),
            },
            {
              key: 'stats',
              label: '统计',
              children: <OkfStatsPanel bundleId={id} />,
            },
            {
              key: 'broken',
              label: '死链',
              children: <OkfBrokenLinksPanel bundleId={id} />,
            },
          ]}
        />
      </Card>

      <OkfFrontmatterDrawer
        node={selectedNode}
        onClose={() => setSelectedNode(null)}
        onPreview={handlePreview}
      />
    </>
  );
}
