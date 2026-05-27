import { useEffect, useState } from 'react';
import { Breadcrumb, Card, List, Spin, Tag } from 'antd';
import { FolderOutlined } from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import { publicDirectoryApi } from '@/api/publicDirectory';

interface PublicDir {
  id: number;
  scope: string;
  department: string;
  displayName: string;
  fixedPath: string;
}

interface FolderItem {
  id: number;
  folderName: string;
}

export default function PublicDirectoriesPage() {
  const { id } = useParams<{ id: string }>();
  const [dirs, setDirs] = useState<PublicDir[]>([]);
  const [dirDetail, setDirDetail] = useState<PublicDir | null>(null);
  const [folders, setFolders] = useState<FolderItem[]>([]);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  useEffect(() => {
    setLoading(true);
    if (id) {
      Promise.all([
        publicDirectoryApi.get(Number(id)).catch(() => ({ data: null })),
        publicDirectoryApi.listSubFolders(Number(id)).catch(() => ({ data: [] })),
      ]).then(([dirRes, folderRes]) => {
        setDirDetail(dirRes.data);
        setFolders(folderRes.data || []);
        setLoading(false);
      });
    } else {
      publicDirectoryApi.listVisible()
        .then((res) => setDirs(res.data || []))
        .catch(() => {})
        .finally(() => setLoading(false));
    }
  }, [id]);

  if (loading) return <Spin style={{ display: 'block', margin: '80px auto' }} />;

  if (id) {
    return (
      <Card title={dirDetail?.displayName || '公共目录详情'}>
        <Breadcrumb
          items={[
            { title: <a onClick={() => navigate('/public')}>公共文件</a> },
            { title: dirDetail?.displayName || '详情' },
          ]}
          style={{ marginBottom: 16 }}
        />
        {dirDetail && (
          <div style={{ marginBottom: 16 }}>
            <Tag color="blue">{dirDetail.scope === 'global' ? '全局' : '部门'}</Tag>
            <span>{dirDetail.fixedPath}</span>
          </div>
        )}
        <List
          dataSource={folders}
          renderItem={(item) => (
            <List.Item>
              <List.Item.Meta
                avatar={<FolderOutlined style={{ fontSize: 24, color: '#1890ff' }} />}
                title={item.folderName}
              />
            </List.Item>
          )}
          locale={{ emptyText: '暂无文件夹' }}
        />
      </Card>
    );
  }

  return (
    <Card title="公共文件">
      <List
        dataSource={dirs}
        renderItem={(item) => (
          <List.Item
            style={{ cursor: 'pointer' }}
            onClick={() => navigate(`/public/${item.id}`)}
          >
            <List.Item.Meta
              avatar={<FolderOutlined style={{ fontSize: 24, color: '#1890ff' }} />}
              title={item.displayName}
              description={<><Tag color="blue">{item.scope === 'global' ? '全局' : '部门'}</Tag><span>{item.fixedPath}</span></>}
            />
          </List.Item>
        )}
        locale={{ emptyText: '暂无公共目录' }}
      />
    </Card>
  );
}
