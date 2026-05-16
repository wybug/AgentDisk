import { Menu } from 'antd';
import {
  FolderOutlined,
  DeleteOutlined,
  ShareAltOutlined,
  TagsOutlined,
  SafetyOutlined,
} from '@ant-design/icons';
import { useNavigate, useLocation } from 'react-router-dom';

const menuItems = [
  { key: '/explorer', icon: <FolderOutlined />, label: '全部文件' },
  { key: '/recycle', icon: <DeleteOutlined />, label: '回收站' },
  { key: '/shares', icon: <ShareAltOutlined />, label: '我的分享' },
  { key: '/tags', icon: <TagsOutlined />, label: '标签搜索' },
  { key: '/permissions', icon: <SafetyOutlined />, label: '权限管理' },
];

export default function AppSidebar() {
  const navigate = useNavigate();
  const location = useLocation();

  const selectedKey = menuItems.find((m) => location.pathname.startsWith(m.key))?.key || '/explorer';

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
