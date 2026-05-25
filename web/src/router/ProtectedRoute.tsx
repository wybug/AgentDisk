import { useEffect } from 'react';
import { useAuthStore } from '@/store/auth';
import { Spin } from 'antd';

export default function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading, checkAuth } = useAuthStore();

  useEffect(() => {
    if (!isAuthenticated && isLoading) {
      checkAuth();
    }
  }, [isAuthenticated, isLoading, checkAuth]);

  if (isLoading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spin size="large"><div style={{ padding: 50 }}>加载中...</div></Spin>
      </div>
    );
  }

  if (!isAuthenticated) {
    window.location.replace('/auth/login');
    return null;
  }

  return <>{children}</>;
}
