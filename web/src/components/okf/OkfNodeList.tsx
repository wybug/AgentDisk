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
import type { OkfNode, OkfTypeCount } from '@/api/types';

interface Props {
  bundleId: number;
  onNodeClick: (node: OkfNode) => void;
}

// OkfNodeList is the "节点" tab on the bundle detail page. It pulls the
// bundle's nodes once (unfiltered) and lets the user filter client-side by
// type and free-text tag. Server-side filtering is also supported via the
// listNodes query params, but client-side keeps the UX snappy for the
// expected < 1000-node range.
export default function OkfNodeList({ bundleId, onNodeClick }: Props) {
  const [nodes, setNodes] = useState<OkfNode[]>([]);
  const [types, setTypes] = useState<OkfTypeCount[]>([]);
  const [typeFilter, setTypeFilter] = useState<string | undefined>(undefined);
  const [tagFilter, setTagFilter] = useState('');
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);

  const loadAll = async () => {
    setLoading(true);
    try {
      const [nodesRes, typesRes] = await Promise.all([
        okfApi.listNodes(bundleId),
        okfApi.aggregateTypes().catch(() => [] as OkfTypeCount[]),
      ]);
      setNodes(nodesRes.nodes || []);
      setTypes(typesRes);
    } finally {
      setLoading(false);
    }
  };

  // Server-side search when the user explicitly clicks search; we keep this
  // separate from the lightweight client-side tag filter above.
  const handleSearch = async () => {
    if (!tagFilter.trim() && !typeFilter) {
      loadAll();
      return;
    }
    setSearching(true);
    try {
      const res = await okfApi.listNodes(bundleId, typeFilter, tagFilter.trim() || undefined);
      setNodes(res.nodes || []);
    } finally {
      setSearching(false);
    }
  };

  useEffect(() => {
    // Defer to avoid cascading renders (react-hooks/set-state-in-effect).
    const t = setTimeout(() => {
      void loadAll();
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
        <Select
          allowClear
          placeholder="按 type 过滤"
          style={{ width: 180 }}
          value={typeFilter}
          onChange={(v) => setTypeFilter(v)}
          options={types.map((t) => ({ value: t.type, label: `${t.type} (${t.count})` }))}
        />
        <Input
          allowClear
          placeholder="按 tag 过滤（精确匹配）"
          style={{ width: 220 }}
          value={tagFilter}
          onChange={(e) => setTagFilter(e.target.value)}
          onPressEnter={handleSearch}
        />
        <Button icon={<SearchOutlined />} loading={searching} onClick={handleSearch}>
          搜索
        </Button>
        <Button icon={<ReloadOutlined />} onClick={loadAll}>
          重置
        </Button>
      </Space>
      <Table
        rowKey="nodeId"
        columns={columns}
        dataSource={nodes}
        loading={loading}
        size="small"
        pagination={{ pageSize: 20, showSizeChanger: false }}
        locale={{ emptyText: '该 bundle 没有节点' }}
      />
    </div>
  );
}
