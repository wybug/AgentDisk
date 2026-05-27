import { useState, useEffect } from 'react';
import { Form, Input, Button, Card, message, Spin } from 'antd';
import { LockOutlined, UserOutlined, SmileOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { adminApi } from '@/api/admin';

export default function AdminSetupPage() {
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(true);
  const navigate = useNavigate();

  useEffect(() => {
    (async () => {
      try {
        const res = await adminApi.checkInitStatus();
        if (res.data?.initialized) {
          navigate('/admin/login', { replace: true });
          return;
        }
      } catch {
        // graceful: show setup page if check fails
      } finally {
        setChecking(false);
      }
    })();
  }, [navigate]);

  const onFinish = async (values: { username: string; password: string; displayName?: string }) => {
    setLoading(true);
    try {
      await adminApi.bootstrap({
        username: values.username,
        password: values.password,
        displayName: values.displayName,
      });

      const loginRes = await adminApi.login(values.username, values.password);
      localStorage.setItem('admin_token', loginRes.data.token);

      message.success('管理员账户创建成功');
      window.location.href = '/admin';
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '初始化失败';
      if (msg.includes('already exists') || msg.includes('403')) {
        navigate('/admin/login', { replace: true });
        return;
      }
      message.error(msg);
    } finally {
      setLoading(false);
    }
  };

  if (checking) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', background: '#f0f2f5' }}>
        <Spin />
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', background: '#f0f2f5' }}>
      <Card title="初始化管理员账户" style={{ width: 420 }}>
        <p style={{ color: '#666', marginBottom: 24 }}>
          首次使用，请创建管理员账户
        </p>
        <Form onFinish={onFinish}>
          <Form.Item name="username" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input prefix={<UserOutlined />} placeholder="用户名" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, min: 6, message: '密码至少6位' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" />
          </Form.Item>
          <Form.Item
            name="confirmPassword"
            dependencies={['password']}
            rules={[
              { required: true, message: '请确认密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('password') === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error('两次密码不一致'));
                },
              }),
            ]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder="确认密码" />
          </Form.Item>
          <Form.Item name="displayName">
            <Input prefix={<SmileOutlined />} placeholder="显示名称（可选）" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} block>
              创建管理员
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
