import { useEffect, useState } from 'react';
import { Card, Col, Progress, Row, Spin, Statistic, Table, Typography } from 'antd';
import { okfApi } from '@/api/okf';
import type { OkfBundleStats } from '@/api/types';

interface Props {
  bundleId: number;
}

// OkfStatsPanel is the "统计" tab on the bundle detail page. One round-trip
// to /bundles/:id/stats, then render 4 numeric tiles + a per-type bar
// (using AntD Progress to avoid pulling in a charting library).
export default function OkfStatsPanel({ bundleId }: Props) {
  const [stats, setStats] = useState<OkfBundleStats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      try {
        const s = await okfApi.stats(bundleId);
        if (!cancelled) setStats(s);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [bundleId]);

  if (loading) return <Spin style={{ display: 'block', margin: '40px auto' }} />;
  if (!stats) return <Typography.Text type="secondary">加载失败</Typography.Text>;

  const maxCount = Math.max(1, ...stats.types.map((t) => t.count));

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="节点总数" value={stats.nodeCount} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="边总数" value={stats.edgeTotal} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="活边"
              value={stats.edgeLive}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="死链"
              value={stats.edgeBroken}
              valueStyle={{ color: stats.edgeBroken > 0 ? '#ff4d4f' : undefined }}
            />
          </Card>
        </Col>
      </Row>

      <Card title="按 type 分布" size="small">
        {stats.types.length === 0 ? (
          <Typography.Text type="secondary">暂无节点</Typography.Text>
        ) : (
          <Table
            rowKey="type"
            size="small"
            pagination={false}
            dataSource={stats.types}
            columns={[
              {
                title: 'type',
                dataIndex: 'type',
                key: 'type',
                width: 120,
                render: (t: string) => <Typography.Text strong>{t}</Typography.Text>,
              },
              {
                title: '节点数',
                dataIndex: 'count',
                key: 'count',
                width: 100,
              },
              {
                title: '占比',
                key: 'bar',
                render: (_: unknown, record) => (
                  <Progress
                    percent={Math.round((record.count / maxCount) * 100)}
                    size="small"
                    format={(p) => `${p}%`}
                  />
                ),
              },
            ]}
          />
        )}
      </Card>
    </div>
  );
}
