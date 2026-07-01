import { useCallback, useEffect, useState } from 'react';
import { App, Button, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { ScanOutlined, ReloadOutlined } from '@ant-design/icons';
import { okfApi } from '@/api/okf';
import type { OkfBrokenLink } from '@/api/types';

interface Props {
  bundleId: number;
}

// OkfBrokenLinksPanel is the "死链" tab. The cursor is the source node ID of
// the last row in the current page; pass it back as the "cursor" query to
// fetch the next page. nextCursor=0 means the bundle has been fully
// enumerated.
export default function OkfBrokenLinksPanel({ bundleId }: Props) {
  const [links, setLinks] = useState<OkfBrokenLink[]>([]);
  const [cursor, setCursor] = useState(0);
  const [nextCursor, setNextCursor] = useState(0);
  const [loading, setLoading] = useState(true);
  const [scanning, setScanning] = useState(false);
  const { message } = App.useApp();

  const load = useCallback(
    async (cur: number) => {
      setLoading(true);
      try {
        const res = await okfApi.listBrokenLinks(bundleId, cur);
        setLinks(res.brokenLinks || []);
        setNextCursor(res.nextCursor || 0);
      } finally {
        setLoading(false);
      }
    },
    [bundleId],
  );

  useEffect(() => {
    // Defer to avoid cascading renders (react-hooks/set-state-in-effect).
    const t = setTimeout(() => {
      void load(0);
    }, 0);
    return () => clearTimeout(t);
  }, [load]);

  const handleScan = async () => {
    setScanning(true);
    try {
      const report = await okfApi.scanBundle(bundleId);
      message.success(`扫描完成：${report.scannedNodes} 个节点，${report.brokenCount} 条死链`);
      setCursor(0);
      load(0);
    } finally {
      setScanning(false);
    }
  };

  const columns: ColumnsType<OkfBrokenLink> = [
    {
      title: '源文件',
      dataIndex: 'srcRelPath',
      key: 'srcRelPath',
      render: (p: string) => (
        <Typography.Text code style={{ fontSize: 12 }}>
          {p}
        </Typography.Text>
      ),
    },
    {
      title: '行',
      dataIndex: 'srcLine',
      key: 'srcLine',
      width: 60,
    },
    {
      title: '链接文字',
      dataIndex: 'linkText',
      key: 'linkText',
      ellipsis: true,
    },
    {
      title: '目标',
      dataIndex: 'dstRelPath',
      key: 'dstRelPath',
      ellipsis: true,
      render: (p: string) => (
        <Typography.Text code type="danger" style={{ fontSize: 12 }}>
          {p}
        </Typography.Text>
      ),
    },
    {
      title: '类型',
      dataIndex: 'linkKind',
      key: 'linkKind',
      width: 90,
      render: (k: string) => <Tag>{k}</Tag>,
    },
    {
      title: '原因',
      dataIndex: 'reason',
      key: 'reason',
      ellipsis: true,
    },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<ScanOutlined />} loading={scanning} onClick={handleScan}>
          扫描死链
        </Button>
        <Button icon={<ReloadOutlined />} onClick={() => load(0)}>
          刷新列表
        </Button>
      </Space>
      <Table
        rowKey={(r) => `${r.srcNodeId}-${r.srcLine}-${r.dstRelPath}`}
        columns={columns}
        dataSource={links}
        loading={loading}
        size="small"
        pagination={
          nextCursor === 0 && cursor === 0
            ? false
            : {
                pageSize: 50,
                current: Math.floor(cursor / 50) + 1,
                total: nextCursor === 0 ? (cursor + 1) * 50 : (cursor + 2) * 50,
                onChange: (page) => {
                  const newCursor = (page - 1) * 50;
                  setCursor(newCursor);
                  load(newCursor);
                },
              }
        }
        locale={{ emptyText: '没有死链' }}
      />
    </div>
  );
}
