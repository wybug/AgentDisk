import { useState, useEffect, useCallback } from 'react';
import { Card, Button, Table, Switch, Popconfirm, message, Space, Typography, Modal, Input } from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined, SafetyOutlined } from '@ant-design/icons';
import { adminApi } from '@/api/admin';
import { startRegistration } from '@simplewebauthn/browser';

const { Text, Title } = Typography;

interface Passkey {
  id: number;
  name: string;
  createdAt: string;
  lastUsedAt: string | null;
}

export default function MFASettingsManager() {
  const [passkeys, setPasskeys] = useState<Passkey[]>([]);
  const [loading, setLoading] = useState(false);
  const [mfaEnabled, setMfaEnabled] = useState(false);
  const [registering, setRegistering] = useState(false);
  const [statusLoading, setStatusLoading] = useState(true);

  const fetchPasskeys = useCallback(async () => {
    setLoading(true);
    try {
      const res = await adminApi.listPasskeys();
      setPasskeys(res.data || []);
    } catch {
      message.error('获取通行密钥列表失败');
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchStatus = useCallback(async () => {
    try {
      const res = await adminApi.getMFAStatus();
      setMfaEnabled(res.data?.mfaEnabled ?? false);
      setStatusLoading(false);
    } catch {
      // ignore
    }
  }, []);

  useEffect(() => { void fetchPasskeys(); void fetchStatus(); }, []); // eslint-disable-line react-hooks/set-state-in-effect

  const handleRegister = async () => {
    setRegistering(true);
    try {
      const beginRes = await adminApi.beginRegistration();
      const { options, sessionKey } = beginRes.data;

      const credential = await startRegistration({ optionsJSON: options.publicKey });

      const name = await new Promise<string>((resolve) => {
        let value = 'Passkey';
        Modal.confirm({
          title: '为通行密钥命名',
          content: (
            <Input
              id="passkey-name-input"
              defaultValue="Passkey"
              placeholder="例如：MacBook Touch ID"
              onChange={(e) => { value = e.target.value || 'Passkey'; }}
            />
          ),
          onOk: () => resolve(value),
          onCancel: () => resolve('Passkey'),
        });
      });

      await adminApi.finishRegistration(sessionKey, JSON.stringify(credential), name);
      message.success('通行密钥注册成功');
      fetchPasskeys();
      fetchStatus();
    } catch (err: unknown) {
      if (err instanceof Error && err.name === 'NotAllowedError') {
        message.info('已取消注册');
      } else {
        message.error(err instanceof Error ? err.message : '注册失败');
      }
    } finally {
      setRegistering(false);
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await adminApi.deletePasskey(id);
      message.success('通行密钥已删除');
      fetchPasskeys();
      fetchStatus();
    } catch {
      message.error('删除失败');
    }
  };

  const handleRename = (record: Passkey) => {
    let value = record.name || 'Passkey';
    Modal.confirm({
      title: '修改通行密钥名称',
      content: (
        <Input
          defaultValue={value}
          placeholder="输入新名称"
          onChange={(e) => { value = e.target.value; }}
        />
      ),
      onOk: async () => {
        if (!value.trim()) {
          message.error('名称不能为空');
          return;
        }
        try {
          await adminApi.renamePasskey(record.id, value.trim());
          message.success('名称已更新');
          fetchPasskeys();
        } catch {
          message.error('重命名失败');
        }
      },
    });
  };

  const handleToggleMFA = async (enabled: boolean) => {
    try {
      await adminApi.setMFAEnabled(enabled);
      setMfaEnabled(enabled);
      message.success(enabled ? 'MFA 已开启' : 'MFA 已关闭');
    } catch (err: unknown) {
      message.error(err instanceof Error ? err.message : '操作失败');
    }
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => name || 'Passkey',
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
    },
    {
      title: '最后使用',
      dataIndex: 'lastUsedAt',
      key: 'lastUsedAt',
      render: (v: string | null) => v || '未使用',
    },
    {
      title: '操作',
      key: 'action',
      render: (_: unknown, record: Passkey) => (
        <Space>
          <Button type="link" icon={<EditOutlined />} onClick={() => handleRename(record)}>
            重命名
          </Button>
          <Popconfirm title="确定删除此通行密钥？" onConfirm={() => handleDelete(record.id)} okText="删除" cancelText="取消">
            <Button type="link" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <Card>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Space>
              <SafetyOutlined style={{ fontSize: 20 }} />
              <Title level={5} style={{ margin: 0 }}>多因素认证 (MFA)</Title>
            </Space>
            <Space>
              <Text>启用 MFA</Text>
              <Switch
                checked={mfaEnabled}
                onChange={handleToggleMFA}
                loading={statusLoading}
                disabled={passkeys.length === 0}
              />
            </Space>
          </div>
          {passkeys.length === 0 && (
            <Text type="secondary">注册至少一个通行密钥后才能启用 MFA</Text>
          )}
          <Button type="primary" icon={<PlusOutlined />} onClick={handleRegister} loading={registering}>
            注册通行密钥
          </Button>
        </Space>
      </Card>

      <Card title="已注册的通行密钥">
        <Table
          columns={columns}
          dataSource={passkeys}
          rowKey="id"
          loading={loading}
          pagination={false}
          locale={{ emptyText: '暂无通行密钥' }}
        />
      </Card>
    </Space>
  );
}
