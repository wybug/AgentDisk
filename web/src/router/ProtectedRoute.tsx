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
    // SSO 回调失败（如 login_required）时不再重试，直接显示提示
    const errorParam = new URLSearchParams(window.location.search).get('error');
    if (errorParam === 'login_required') {
      return (
        <div style={{ display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center', height: '100vh', gap: 16 }}>
          <h2>登录已过期</h2>
          <a href="/auth/login">重新登录</a>
        </div>
      );
    }

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
