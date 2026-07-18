import { useEffect, useState } from 'react';
import {
  Table,
  Input,
  Select,
  Space,
  Button,
  Tag,
  Typography,
  Tooltip,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { ReloadOutlined, SearchOutlined, WarningOutlined } from '@ant-design/icons';
import { okfApi } from '@/api/okf';
import type { OkfNode, OkfNodesResult, OkfTypeCount } from '@/api/types';

// In the authed path these default to okfApi. The share path passes
// share-mode fetchers that route through /v1/disk/share/:code/... instead.
export interface OkfNodeListFetchers {
  listNodes: (
    bundleId: number,
    type?: string,
    tag?: string,
    cursor?: number,
    limit?: number,
  ) => Promise<OkfNodesResult>;
  aggregateTypes?: () => Promise<OkfTypeCount[]>;
}

interface Props {
  bundleId: number;
  onNodeClick: (node: OkfNode) => void;
  fetchers?: OkfNodeListFetchers;
}

const PAGE_SIZE = 50;

// OkfNodeList is the "节点" tab on the bundle detail page. It loads nodes in
// pages (server-side, via the cursor) and appends with a "加载更多" button — a
// full client-side load janks on bundles with thousands of nodes. type/tag
// filters re-run the query from page 1.
export default function OkfNodeList({ bundleId, onNodeClick, fetchers }: Props) {
  const listNodesFn = fetchers?.listNodes ?? okfApi.listNodes;
  const aggregateTypesFn = fetchers?.aggregateTypes;

  const [nodes, setNodes] = useState<OkfNode[]>([]);
  const [types, setTypes] = useState<OkfTypeCount[]>([]);
  const [typeFilter, setTypeFilter] = useState<string | undefined>(undefined);
  const [tagFilter, setTagFilter] = useState('');
  const [nextCursor, setNextCursor] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);

  // loadFirst resets to page 1 with the given filters (undefined = unfiltered).
  const loadFirst = async (type?: string, tag?: string) => {
    setLoading(true);
    try {
      const [nodesRes, typesRes] = await Promise.all([
        listNodesFn(bundleId, type, tag, 0, PAGE_SIZE),
        aggregateTypesFn
          ? aggregateTypesFn().catch(() => [] as OkfTypeCount[])
          : Promise.resolve([] as OkfTypeCount[]),
      ]);
      setNodes(nodesRes.nodes || []);
      setNextCursor(nodesRes.nextCursor || 0);
      setTypes(typesRes);
    } finally {
      setLoading(false);
    }
  };

  const loadMore = async () => {
    if (!nextCursor) return;
    setLoadingMore(true);
    try {
      const res = await listNodesFn(
        bundleId,
        typeFilter,
        tagFilter.trim() || undefined,
        nextCursor,
        PAGE_SIZE,
      );
      setNodes((prev) => [...prev, ...(res.nodes || [])]);
      setNextCursor(res.nextCursor || 0);
    } finally {
      setLoadingMore(false);
    }
  };

  const handleSearch = () => {
    void loadFirst(typeFilter, tagFilter.trim() || undefined);
  };

  const reset = () => {
    setTypeFilter(undefined);
    setTagFilter('');
    void loadFirst();
  };

  useEffect(() => {
    // Defer to avoid cascading renders (react-hooks/set-state-in-effect).
    const t = setTimeout(() => {
      void loadFirst();
    }, 0);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bundleId]);

  const columns: ColumnsType<OkfNode> = [
    {
      title: '标题',
      dataIndex: 'title',
      key: 'title',
      render: (_: string, record) => (
        <Space>
          <a onClick={() => onNodeClick(record)}>{record.title || record.relPath}</a>
          {record.hasBrokenLink && (
            <Tooltip title="存在死链">
              <WarningOutlined style={{ color: '#ff4d4f' }} />
            </Tooltip>
          )}
        </Space>
      ),
    },
    {
      title: 'type',
      dataIndex: 'type',
      key: 'type',
      width: 120,
      render: (t: string) => <Tag color="blue">{t}</Tag>,
    },
    {
      title: 'tags',
      dataIndex: 'tags',
      key: 'tags',
      render: (tags: string[]) =>
        tags.length === 0 ? (
          <Typography.Text type="secondary">-</Typography.Text>
        ) : (
          <Space size={4} wrap>
            {tags.map((t) => (
              <Tag key={t}>{t}</Tag>
            ))}
          </Space>
        ),
    },
    {
      title: 'relPath',
      dataIndex: 'relPath',
      key: 'relPath',
      ellipsis: true,
      render: (p: string) => (
        <Typography.Text code style={{ fontSize: 12 }}>
          {p}
        </Typography.Text>
      ),
    },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        {aggregateTypesFn && (
          <Select
            allowClear
            placeholder="按 type 过滤"
            style={{ width: 180 }}
            value={typeFilter}
            onChange={(v) => setTypeFilter(v)}
            options={types.map((t) => ({ value: t.type, label: `${t.type} (${t.count})` }))}
          />
        )}
        <Input
          allowClear
          placeholder="按 tag 过滤（精确匹配）"
          style={{ width: 220 }}
          value={tagFilter}
          onChange={(e) => setTagFilter(e.target.value)}
          onPressEnter={handleSearch}
        />
        <Button icon={<SearchOutlined />} loading={loading} onClick={handleSearch}>
          搜索
        </Button>
        <Button icon={<ReloadOutlined />} onClick={reset}>
          重置
        </Button>
      </Space>
      <Table
        rowKey="nodeId"
        columns={columns}
        dataSource={nodes}
        loading={loading}
        size="small"
        pagination={false}
        locale={{ emptyText: '该 bundle 没有节点' }}
      />
      {nextCursor !== 0 && (
        <div style={{ textAlign: 'center', marginTop: 12 }}>
          <Button loading={loadingMore} onClick={loadMore}>
            加载更多
          </Button>
        </div>
      )}
    </div>
  );
}
