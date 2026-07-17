import { useState } from 'react';
import { Button, Card, Empty, Input, List, Select, Space, Spin, Tag, Typography, App } from 'antd';
import { ClearOutlined, SearchOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { okfApi } from '@/api/okf';
import type { OkfNode } from '@/api/types';

interface Props {
  bundleId: number;
  onNodeClick: (n: OkfNode) => void;
}

const PAGE_SIZE = 20;

// OkfSearchPanel wires the backend full-text search (POST /v1/disk/okf/search)
// into the bundle detail page. Until this component okfApi.search had zero call
// sites — the knowledge base had no way to search node titles/descriptions.
//
// Searches are bundle-scoped (bundleId) with an optional type filter sourced
// from the global type rollup. Results stream via cursor "load more"; clicking a
// row opens the same frontmatter drawer the node list uses.
export default function OkfSearchPanel({ bundleId, onNodeClick }: Props) {
  const [committed, setCommitted] = useState('');
  const [type, setType] = useState<string | undefined>(undefined);
  const [results, setResults] = useState<OkfNode[]>([]);
  const [nextCursor, setNextCursor] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searched, setSearched] = useState(false);
  const { message } = App.useApp();

  // Type-filter options come from the global type rollup (cheap, cached).
  const { data: types } = useQuery({
    queryKey: ['okf-types'],
    queryFn: () => okfApi.aggregateTypes(),
    staleTime: 60_000,
  });

  const doSearch = async (q: string, cursor: number, prev: OkfNode[]) => {
    if (!q.trim()) return;
    setLoading(true);
    setError(null);
    try {
      const page = await okfApi.search({
        query: q.trim(),
        bundleId,
        type,
        limit: PAGE_SIZE,
        cursor,
      });
      setResults([...prev, ...page.nodes]);
      setNextCursor(page.nextCursor || 0);
      setSearched(true);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      message.error('搜索失败: ' + msg);
    } finally {
      setLoading(false);
    }
  };

  const onSearch = (q: string) => {
    setCommitted(q);
    doSearch(q, 0, []);
  };

  const loadMore = () => doSearch(committed, nextCursor, results);

  const clear = () => {
    setCommitted('');
    setResults([]);
    setNextCursor(0);
    setSearched(false);
    setError(null);
  };

  return (
    <Card
      size="small"
      title={
        <Space>
          <SearchOutlined /> 搜索节点
        </Space>
      }
      extra={
        searched ? (
          <Button size="small" type="text" icon={<ClearOutlined />} onClick={clear}>
            清除
          </Button>
        ) : null
      }
    >
      <div style={{ display: 'flex', gap: 8, marginBottom: 12 }}>
        <Select
          allowClear
          placeholder="按 type 过滤"
          style={{ width: 200 }}
          value={type}
          onChange={(v) => setType(v)}
          options={(types ?? []).map((t) => ({ value: t.type, label: `${t.type} (${t.count})` }))}
        />
        <Input.Search
          placeholder="搜索节点标题 / 描述（回车或点击搜索）"
          allowClear
          enterButton="搜索"
          loading={loading}
          onSearch={onSearch}
          style={{ flex: 1 }}
        />
      </div>

      {loading && results.length === 0 && <Spin style={{ display: 'block', margin: '24px auto' }} />}
      {error && <Typography.Text type="danger">{error}</Typography.Text>}
      {!loading && searched && results.length === 0 && (
        <Empty description="没有匹配的节点" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      )}
      {results.length > 0 && (
        <>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            共加载 {results.length} 条{nextCursor !== 0 ? '（还有更多）' : ''}
          </Typography.Text>
          <List
            style={{ marginTop: 8 }}
            dataSource={results}
            renderItem={(n) => (
              <List.Item style={{ cursor: 'pointer', padding: '8px 0' }} onClick={() => onNodeClick(n)}>
                <Space direction="vertical" size={2} style={{ width: '100%' }}>
                  <Space>
                    {n.type && <Tag color="blue">{n.type}</Tag>}
                    <Typography.Text strong>{n.title || n.relPath}</Typography.Text>
                    {n.hasBrokenLink && <Tag color="red">死链</Tag>}
                  </Space>
                  {n.description && (
                    <Typography.Text type="secondary" ellipsis={{ tooltip: n.description }}>
                      {n.description}
                    </Typography.Text>
                  )}
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {n.relPath}
                  </Typography.Text>
                </Space>
              </List.Item>
            )}
          />
          {nextCursor !== 0 && (
            <div style={{ textAlign: 'center', marginTop: 12 }}>
              <Button loading={loading} onClick={loadMore}>
                加载更多
              </Button>
            </div>
          )}
        </>
      )}
      {!searched && !loading && (
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          输入关键词搜索本 bundle 的节点（匹配标题与描述）。
        </Typography.Text>
      )}
    </Card>
  );
}
