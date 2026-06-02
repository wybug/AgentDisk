import { useEffect } from 'react';
import { useAuthStore } from '@/store/auth';
import { Spin } from 'antd';
import axios from 'axios';

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
    axios.get('/auth/status').then((res) => {
      if (res.data?.data?.oauth2 === false) {
        window.location.replace('/auth/unavailable');
      } else {
        window.location.replace('/auth/login');
      }
    }).catch(() => {
      window.location.replace('/auth/unavailable');
    });
    return null;
  }

  return <>{children}</>;
}
