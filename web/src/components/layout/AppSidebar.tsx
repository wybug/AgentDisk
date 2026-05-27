import { Menu } from 'antd';
import {
  FolderOutlined,
  DeleteOutlined,
  ShareAltOutlined,
  TagsOutlined,
  SafetyOutlined,
  GlobalOutlined,
} from '@ant-design/icons';
import { useNavigate, useLocation } from 'react-router-dom';

const menuItems = [
  { key: '/explorer', icon: <FolderOutlined />, label: '全部文件' },
  { type: 'divider' as const },
  { key: '/public', icon: <GlobalOutlined />, label: '公共文件' },
  { type: 'divider' as const },
  { key: '/recycle', icon: <DeleteOutlined />, label: '回收站' },
  { key: '/shares', icon: <ShareAltOutlined />, label: '我的分享' },
  { key: '/tags', icon: <TagsOutlined />, label: '标签搜索' },
  { key: '/permissions', icon: <SafetyOutlined />, label: '权限管理' },
];

export default function AppSidebar() {
  const navigate = useNavigate();
  const location = useLocation();

  const selectedKey = menuItems
    .filter((m) => 'key' in m)
    .find((m) => location.pathname.startsWith(m.key as string))?.key as string || '/explorer';

  return (
    <Menu
      mode="inline"
      selectedKeys={[selectedKey]}
      items={menuItems}
      onClick={({ key }) => navigate(key)}
      style={{ height: '100%', borderRight: 0 }}
    />
  );
}
