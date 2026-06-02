import { Button, Result } from 'antd';

export default function OAuth2NotConfiguredPage() {
  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', background: '#f5f5f5' }}>
      <Result
        status="warning"
        title="OAuth2 登录未配置"
        subTitle="管理员尚未配置 OAuth2 认证服务，暂无法通过网页登录。请联系管理员在后台完成 OAuth2 配置。"
        extra={[
          <Button key="admin" type="primary" href="/admin/login">
            管理员登录
          </Button>,
          <Button key="retry" href="/auth/login">
            重试
          </Button>,
        ]}
      />
    </div>
  );
}
