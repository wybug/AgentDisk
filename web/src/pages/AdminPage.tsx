import { useState } from 'react';
import { Layout, Menu, Button, message } from 'antd';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import {
  FolderOutlined,
  KeyOutlined,
  TeamOutlined,
  SettingOutlined,
  LogoutOutlined,
} from '@ant-design/icons';

const { Sider, Content, Header } = Layout;

const menuItems = [
  { key: '/admin/public-dirs', icon: <FolderOutlined />, label: '公共目录' },
  { key: '/admin/api-keys', icon: <KeyOutlined />, label: 'API Key' },
  { key: '/admin/users', icon: <TeamOutlined />, label: '管理员' },
  { key: '/admin/oauth2', icon: <SettingOutlined />, label: 'OAuth2 配置' },
];

export default function AdminPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const [collapsed] = useState(false);

  const handleLogout = () => {
    localStorage.removeItem('admin_token');
    message.success('已退出');
    navigate('/admin/login');
  };

  const selectedKey = menuItems.find((m) => location.pathname.startsWith(m.key))?.key || '/admin/public-dirs';

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '0 24px', background: '#001529' }}>
        <span style={{ color: '#fff', fontSize: 18, fontWeight: 'bold' }}>AgentDisk 管理后台</span>
        <Button type="text" icon={<LogoutOutlined />} onClick={handleLogout} style={{ color: '#fff' }}>
          退出
        </Button>
      </Header>
      <Layout>
        <Sider width={200} theme="light" collapsible collapsed={collapsed}>
          <Menu
            mode="inline"
            selectedKeys={[selectedKey]}
            items={menuItems}
            onClick={({ key }) => navigate(key)}
            style={{ height: '100%', borderRight: 0 }}
          />
        </Sider>
        <Content style={{ padding: 24, background: '#fff', margin: 0, minHeight: 280 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
