import { Tree, Spin } from 'antd';
import type { TreeProps } from 'antd';
import { FolderOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { folderApi } from '@/api/folder';
import { useNavigate } from 'react-router-dom';
import type { DiskFolder } from '@/api/types';

function buildTree(folders: DiskFolder[]): TreeProps['treeData'] {
  const map = new Map<number, DiskFolder[]>();
  for (const f of folders) {
    const children = map.get(f.parentId) || [];
    children.push(f);
    map.set(f.parentId, children);
  }

  function getChildren(parentId: number): TreeProps['treeData'] {
    const children = map.get(parentId) || [];
    return children.map((f) => ({
      key: f.id,
      title: f.folderName,
      icon: <FolderOutlined />,
      children: getChildren(f.id),
    }));
  }

  return getChildren(0);
}

export default function FolderTree() {
  const navigate = useNavigate();

  const { data: folders, isLoading } = useQuery({
    queryKey: ['folders-tree'],
    queryFn: async () => {
      const rootFolders = await folderApi.list(0);
      const allFolders = [...rootFolders];
      for (const f of rootFolders) {
        try {
          const sub = await folderApi.list(f.id);
          allFolders.push(...sub);
        } catch {
          // skip on error
        }
      }
      return allFolders;
    },
    staleTime: 30 * 1000,
  });

  if (isLoading) return <Spin size="small" />;

  return (
    <Tree
      showIcon
      defaultExpandAll
      treeData={buildTree(folders || [])}
      onSelect={(keys) => {
        if (keys.length > 0) {
          navigate(`/explorer/${keys[0]}`);
        }
      }}
      style={{ background: 'transparent' }}
    />
  );
}
