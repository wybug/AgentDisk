import { useState } from 'react';
import { Input, Button, Table, message } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { tagApi } from '@/api/tag';
import { formatFileSize, formatDate } from '@/utils/format';
import type { DiskFile } from '@/api/types';

export default function TagsPage() {
  const [tagInput, setTagInput] = useState('');
  const [searchTags, setSearchTags] = useState('');

  const { data: files = [], isLoading } = useQuery({
    queryKey: ['tag-search', searchTags],
    queryFn: () => tagApi.search(searchTags),
    enabled: !!searchTags,
  });

  const handleSearch = () => {
    if (!tagInput.trim()) {
      message.warning('请输入标签');
      return;
    }
    setSearchTags(tagInput.trim());
  };

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}>标签搜索</h2>
      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        <Input
          value={tagInput}
          onChange={(e) => setTagInput(e.target.value)}
          placeholder="输入标签，多个标签用逗号分隔"
          onPressEnter={handleSearch}
          style={{ maxWidth: 400 }}
        />
        <Button type="primary" icon={<SearchOutlined />} onClick={handleSearch}>
          搜索
        </Button>
      </div>

      <Table
        dataSource={files}
        loading={isLoading}
        rowKey="id"
        pagination={false}
        columns={[
          { title: '文件名', dataIndex: 'fileName' },
          { title: '大小', dataIndex: 'fileSize', width: 100, render: formatFileSize },
          { title: '类型', dataIndex: 'fileType', width: 80 },
          {
            title: '标签',
            dataIndex: 'tags',
            width: 200,
            render: (tags: string) => tags?.split(',').filter(Boolean).map((t) => (
              <span key={t} style={{ marginRight: 4 }}>{t}</span>
            )) || '-',
          },
          { title: '修改时间', dataIndex: 'updatedAt', width: 170, render: formatDate },
        ]}
      />
    </div>
  );
}
