import { Breadcrumb } from 'antd';
import { HomeOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';

interface BreadcrumbItem {
  id: number;
  name: string;
}

interface Props {
  items: BreadcrumbItem[];
}

export default function BreadcrumbNav({ items }: Props) {
  const navigate = useNavigate();

  return (
    <Breadcrumb
      style={{ marginBottom: 16 }}
      items={[
        {
          title: (
            <a onClick={() => navigate('/explorer')}>
              <HomeOutlined /> 全部文件
            </a>
          ),
        },
        ...items.map((item, idx) => ({
          title: idx === items.length - 1 ? (
            item.name
          ) : (
            <a onClick={() => navigate(`/explorer/${item.id}`)}>{item.name}</a>
          ),
        })),
      ]}
    />
  );
}
