import { Layout } from 'antd';
import { Outlet } from 'react-router-dom';
import AppSidebar from './AppSidebar';
import AppHeader from './AppHeader';

const { Content, Sider } = Layout;

export default function AppLayout() {
  return (
    <Layout style={{ minHeight: '100vh' }}>
      <AppHeader />
      <Layout>
        <Sider
          width={200}
          style={{ background: '#fff' }}
          breakpoint="lg"
          collapsedWidth={0}
        >
          <AppSidebar />
        </Sider>
        <Layout style={{ padding: '24px' }}>
          <Content
            style={{
              background: '#fff',
              padding: 24,
              borderRadius: 8,
              minHeight: 280,
            }}
          >
            <Outlet />
          </Content>
        </Layout>
      </Layout>
    </Layout>
  );
}
