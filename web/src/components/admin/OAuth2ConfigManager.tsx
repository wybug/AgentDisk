import { useEffect, useState } from 'react';
import { Form, Input, Button, Switch, message, Card, Space } from 'antd';
import { adminApi } from '@/api/admin';

export default function OAuth2ConfigManager() {
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm();
  const [testResult, setTestResult] = useState<string>('');

  const load = async () => {
    try {
      const res = await adminApi.getOAuth2Config();
      if (res.data) {
        form.setFieldsValue(res.data);
      }
    } catch { /* no config yet */ }
  };

  useEffect(() => { load(); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleSave = async (values: Record<string, unknown>) => {
    setLoading(true);
    try {
      await adminApi.updateOAuth2Config(values);
      message.success('保存成功');
    } catch (err: unknown) {
      message.error(err instanceof Error ? err.message : '保存失败');
    }
    setLoading(false);
  };

  const handleTest = async () => {
    setTestResult('测试中...');
    try {
      const res = await adminApi.testOAuth2Config();
      setTestResult(res.data?.status === 'ok' ? '连接成功' : `异常: ${res.data?.message}`);
    } catch (err: unknown) {
      setTestResult(`失败: ${err instanceof Error ? err.message : '未知错误'}`);
    }
  };

  return (
    <Card title="OAuth2 认证配置">
      <Form form={form} onFinish={handleSave} layout="vertical">
        <Form.Item name="enabled" label="启用" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="clientId" label="Client ID">
          <Input />
        </Form.Item>
        <Form.Item name="clientSecret" label="Client Secret">
          <Input.Password />
        </Form.Item>
        <Form.Item name="issuerUrl" label="Issuer URL" extra="OAuth2 提供方基础地址，如 http://localhost:3100">
          <Input placeholder="http://localhost:3100" />
        </Form.Item>
        <Form.Item name="redirectUrl" label="Redirect URL">
          <Input placeholder="http://localhost:9100/auth/callback" />
        </Form.Item>
        <Form.Item name="scopes" label="Scopes（逗号分隔）">
          <Input placeholder="openid,profile" />
        </Form.Item>
        <Space>
          <Button type="primary" htmlType="submit" loading={loading}>保存</Button>
          <Button onClick={handleTest}>测试连接</Button>
        </Space>
      </Form>
      {testResult && <p style={{ marginTop: 16 }}>{testResult}</p>}
    </Card>
  );
}
