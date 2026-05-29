import { useState, useEffect } from 'react';
import { Form, Input, Button, Card, message, Typography, Space } from 'antd';
import { LockOutlined, UserOutlined, SafetyOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { adminApi } from '@/api/admin';
import { startAuthentication } from '@simplewebauthn/browser';

const { Text } = Typography;

export default function AdminLoginPage() {
  const [loading, setLoading] = useState(false);
  const [mfaRequired, setMfaRequired] = useState(false);
  const [mfaLoading, setMfaLoading] = useState(false);
  const [sessionToken, setSessionToken] = useState('');
  const [mfaUsername, setMfaUsername] = useState('');
  const navigate = useNavigate();

  useEffect(() => {
    (async () => {
      try {
        const res = await adminApi.checkInitStatus();
        if (!res.data?.initialized) {
          navigate('/admin/setup', { replace: true });
        }
      } catch {
        // graceful: show login page if check fails
      }
    })();
  }, [navigate]);

  const onFinish = async (values: { username: string; password: string }) => {
    setLoading(true);
    try {
      const res = await adminApi.login(values.username, values.password);

      if (res.data?.mfaRequired) {
        setMfaRequired(true);
        setSessionToken(res.data.sessionToken);
        setMfaUsername(res.data.username);
        return;
      }

      localStorage.setItem('admin_token', res.data.token);
      window.location.href = '/admin';
    } catch (err: unknown) {
      message.error(err instanceof Error ? err.message : '登录失败');
    } finally {
      setLoading(false);
    }
  };

  const handleMFAVerify = async () => {
    setMfaLoading(true);
    try {
      const beginRes = await adminApi.beginMFALogin(sessionToken);
      const { options, sessionKey } = beginRes.data;

      const credential = await startAuthentication({ optionsJSON: options.publicKey });

      const finishRes = await adminApi.finishMFALogin(sessionKey, JSON.stringify(credential));

      localStorage.setItem('admin_token', finishRes.data.token);
      window.location.href = '/admin';
    } catch (err: unknown) {
      message.error(err instanceof Error ? err.message : '身份验证失败');
    } finally {
      setMfaLoading(false);
    }
  };

  if (mfaRequired) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', background: '#f0f2f5' }}>
        <Card title="验证身份" style={{ width: 400 }}>
          <Space direction="vertical" size="large" style={{ width: '100%', textAlign: 'center' }}>
            <SafetyOutlined style={{ fontSize: 48, color: '#1890ff' }} />
            <div>
              <Text>请使用通行密钥验证身份</Text>
              <br />
              <Text type="secondary">用户: {mfaUsername}</Text>
            </div>
            <Button type="primary" icon={<SafetyOutlined />} loading={mfaLoading} onClick={handleMFAVerify} block size="large">
              验证通行密钥
            </Button>
            <Button type="link" onClick={() => { setMfaRequired(false); setSessionToken(''); }}>
              返回登录
            </Button>
          </Space>
        </Card>
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', background: '#f0f2f5' }}>
      <Card title="AgentDisk 管理后台" style={{ width: 400 }}>
        <Form onFinish={onFinish}>
          <Form.Item name="username" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input prefix={<UserOutlined />} placeholder="用户名" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} block>
              登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
