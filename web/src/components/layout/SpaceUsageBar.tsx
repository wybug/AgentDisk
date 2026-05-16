import { Progress, Tooltip, Typography } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { spaceApi } from '@/api/space';
import { formatFileSize } from '@/utils/format';

export default function SpaceUsageBar() {
  const { data: space } = useQuery({
    queryKey: ['space'],
    queryFn: () => spaceApi.get(),
    staleTime: 5 * 60 * 1000,
  });

  if (!space) return null;

  const percent = space.totalQuota > 0 ? Math.round((space.usedQuota / space.totalQuota) * 100) : 0;
  const color = percent > 90 ? '#ff4d4f' : percent > 70 ? '#faad14' : '#1890ff';

  return (
    <Tooltip title={`${formatFileSize(space.usedQuota)} / ${formatFileSize(space.totalQuota)}`}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <Progress
          percent={percent}
          size="small"
          strokeColor={color}
          style={{ width: 120, marginBottom: 0 }}
          showInfo={false}
        />
        <Typography.Text type="secondary" style={{ fontSize: 12, whiteSpace: 'nowrap' }}>
          {formatFileSize(space.usedQuota)} / {formatFileSize(space.totalQuota)}
        </Typography.Text>
      </div>
    </Tooltip>
  );
}
