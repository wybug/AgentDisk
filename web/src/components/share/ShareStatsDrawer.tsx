import { Card, Col, Drawer, Empty, Row, Spin, Statistic, Table, Typography } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { shareApi } from '@/api/share';
import type { ShareAccessLog } from '@/api/types';
import { formatDate } from '@/utils/format';

interface Props {
  shareId: number | null;
  onClose: () => void;
}

// ShareStatsDrawer shows a share's access analytics to its owner: visit count,
// distinct visitor IPs, last access time, and the most-recent access-log rows.
// Visitor IPs are already masked by the backend (last octet/group zeroed). One
// round-trip on open, matching the OkfStatsPanel single-fetch pattern.
export default function ShareStatsDrawer({ shareId, onClose }: Props) {
  const open = shareId !== null;

  const { data: stats, isLoading } = useQuery({
    queryKey: ['share-stats', shareId],
    queryFn: () => shareApi.stats(shareId as number),
    enabled: open,
  });

  return (
    <Drawer
      title="访问记录"
      placement="right"
      width={520}
      open={open}
      onClose={onClose}
      destroyOnClose
    >
      {isLoading || !stats ? (
        <Spin style={{ display: 'block', margin: '40px auto' }} />
      ) : (
        <>
          <Row gutter={16} style={{ marginBottom: 24 }}>
            <Col span={8}>
              <Card>
                <Statistic title="访问次数" value={stats.visitCount} />
              </Card>
            </Col>
            <Col span={8}>
              <Card>
                <Statistic title="独立访客" value={stats.uniqueIPs} />
              </Card>
            </Col>
            <Col span={8}>
              <Card>
                <Statistic
                  title="最近访问"
                  value={stats.lastAccessAt ? formatDate(stats.lastAccessAt) : '—'}
                  valueStyle={{ fontSize: 14 }}
                />
              </Card>
            </Col>
          </Row>

          <Typography.Text type="secondary">最近访问记录（最多 50 条）</Typography.Text>
          {stats.recentLogs.length === 0 ? (
            <Empty style={{ marginTop: 24 }} description="暂无访问记录" />
          ) : (
            <Table<ShareAccessLog>
              rowKey={(r) => `${r.createdAt}|${r.visitorIP}`}
              size="small"
              pagination={false}
              style={{ marginTop: 8 }}
              dataSource={stats.recentLogs}
              columns={[
                {
                  title: '时间',
                  dataIndex: 'createdAt',
                  render: (v: string) => formatDate(v),
                },
                { title: 'IP', dataIndex: 'visitorIP', width: 120 },
                {
                  title: 'User-Agent',
                  dataIndex: 'userAgent',
                  render: (v: string) => (
                    <Typography.Text style={{ fontSize: 12 }} ellipsis={{ tooltip: v }}>
                      {v}
                    </Typography.Text>
                  ),
                },
              ]}
            />
          )}
        </>
      )}
    </Drawer>
  );
}
