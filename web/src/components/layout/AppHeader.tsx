import { Layout, Dropdown, Typography } from 'antd';
import { UserOutlined, LogoutOutlined } from '@ant-design/icons';
import { useAuthStore } from '@/store/auth';
import SpaceUsageBar from './SpaceUsageBar';
import { APP_NAME } from '@/utils/constants';

const { Header } = Layout;

export default function AppHeader() {
  const { logout, userId } = useAuthStore();

  const menuItems = [
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      onClick: logout,
    },
  ];

  return (
    <Header
      style={{
        background: '#fff',
        padding: '0 24px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        borderBottom: '1px solid #f0f0f0',
        boxShadow: '0 1px 4px rgba(0,0,0,0.05)',
      }}
    >
      <Typography.Title level={4} style={{ margin: 0 }}>
        {APP_NAME}
      </Typography.Title>

      <div style={{ display: 'flex', alignItems: 'center', gap: 24 }}>
        <SpaceUsageBar />
        <Dropdown menu={{ items: menuItems }} placement="bottomRight">
          <a style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <UserOutlined />
            {userId || '用户'}
          </a>
        </Dropdown>
      </div>
    </Header>
  );
}
