import { useMemo, useState } from 'react';
import { Button, Card, Input, List, Select, Space, Spin, Tag, Typography, App } from 'antd';
import { ReloadOutlined, PlusOutlined, ClusterOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { okfApi } from '@/api/okf';
import RegisterBundleModal from '@/components/okf/RegisterBundleModal';
import type { OkfBundle } from '@/api/types';

// OkfBundlesPage is the OKF landing route. Lists every bundle visible to the
// caller as a clickable card. Top toolbar exposes refresh + register; the
// register modal pulls the visible public directories via the existing
// publicDirectory API and POSTs to /okf/bundles/register on submit.
export default function OkfBundlesPage() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [registerOpen, setRegisterOpen] = useState(false);
  const { message } = App.useApp();

  const { data: bundles = [], isLoading } = useQuery({
    queryKey: ['okf-bundles'],
    queryFn: () => okfApi.listBundles(),
  });

  // Client-side search + sort — bundle counts are small, so this stays snappy
  // without a server query. Search matches title/description; sort by node
  // count (default), recency, or title.
  const [query, setQuery] = useState('');
  const [sortBy, setSortBy] = useState<'nodes' | 'updated' | 'title'>('nodes');
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    const filtered = q
      ? bundles.filter(
          (b) =>
            (b.title || '').toLowerCase().includes(q) ||
            (b.description || '').toLowerCase().includes(q),
        )
      : bundles;
    return [...filtered].sort((a, b) => {
      if (sortBy === 'title') return (a.title || '').localeCompare(b.title || '');
      if (sortBy === 'updated') {
        return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime();
      }
      return b.nodeCount - a.nodeCount; // 'nodes', most first
    });
  }, [bundles, query, sortBy]);

  const handleRefresh = () => {
    queryClient.invalidateQueries({ queryKey: ['okf-bundles'] });
  };

  const handleRefreshBundle = async (id: number) => {
    try {
      await okfApi.refreshBundle(id);
      message.success('Bundle 已刷新');
      queryClient.invalidateQueries({ queryKey: ['okf-bundles'] });
    } catch {
      message.error('刷新失败');
    }
  };

  return (
    <>
      <Card
        title={
          <Space>
            <ClusterOutlined />
            <span>OKF 知识库</span>
          </Space>
        }
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={handleRefresh}>
              刷新
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setRegisterOpen(true)}>
              注册 Bundle
            </Button>
          </Space>
        }
      >
        <Space style={{ marginBottom: 16 }} wrap>
          <Input
            allowClear
            placeholder="搜索 bundle 标题 / 描述"
            style={{ width: 260 }}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <Select
            style={{ width: 160 }}
            value={sortBy}
            onChange={(v) => setSortBy(v)}
            options={[
              { value: 'nodes', label: '节点数最多' },
              { value: 'updated', label: '最近更新' },
              { value: 'title', label: '标题' },
            ]}
          />
        </Space>
        {isLoading ? (
          <Spin style={{ display: 'block', margin: '80px auto' }} />
        ) : (
          <List
            grid={{ gutter: 16, column: 3, xs: 1, sm: 2, md: 3 }}
            dataSource={visible}
            locale={{ emptyText: query ? '没有匹配的 bundle' : '暂无 OKF bundle，点击右上角注册一个' }}
            renderItem={(b: OkfBundle) => (
              <List.Item>
                <Card
                  hoverable
                  size="small"
                  title={
                    <Space>
                      <a onClick={() => navigate(`/okf/${b.bundleId}`)}>{b.title || `Bundle #${b.bundleId}`}</a>
                      <Tag color={b.status === 'active' ? 'green' : 'default'}>{b.status}</Tag>
                    </Space>
                  }
                  onClick={() => navigate(`/okf/${b.bundleId}`)}
                >
                  <Space direction="vertical" size={4} style={{ width: '100%' }}>
                    <Space size={4} wrap>
                      <Tag color="blue">v{b.okfVersion}</Tag>
                      <Tag>{b.nodeCount} 节点</Tag>
                      <Tag>{b.edgeCount} 边</Tag>
                    </Space>
                    {b.description && (
                      <Typography.Paragraph
                        type="secondary"
                        ellipsis={{ rows: 2 }}
                        style={{ marginBottom: 0 }}
                      >
                        {b.description}
                      </Typography.Paragraph>
                    )}
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      更新于 {new Date(b.updatedAt).toLocaleString()}
                    </Typography.Text>
                    <Button
                      size="small"
                      icon={<ReloadOutlined />}
                      onClick={(e) => {
                        e.stopPropagation();
                        void handleRefreshBundle(b.bundleId);
                      }}
                    >
                      刷新
                    </Button>
                  </Space>
                </Card>
              </List.Item>
            )}
          />
        )}
      </Card>

      <RegisterBundleModal
        open={registerOpen}
        onClose={() => setRegisterOpen(false)}
        onRegistered={() => queryClient.invalidateQueries({ queryKey: ['okf-bundles'] })}
      />
    </>
  );
}
