import { Modal, Tag, Input, Button, Space, message } from 'antd';
import { PlusOutlined, CloseOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { tagApi } from '@/api/tag';
import { useQueryClient } from '@tanstack/react-query';
import type { DiskFile } from '@/api/types';

interface Props {
  file: DiskFile | null;
  open: boolean;
  onClose: () => void;
}

export default function TagInput({ file, open, onClose }: Props) {
  const [newTag, setNewTag] = useState('');
  const [tags, setTags] = useState<string[]>([]);
  const queryClient = useQueryClient();

  useState(() => {
    if (file?.tags) {
      setTags(file.tags.split(',').filter(Boolean));
    } else {
      setTags([]);
    }
  });

  const handleAdd = async () => {
    if (!file || !newTag.trim()) return;
    try {
      await tagApi.bind(file.id, newTag.trim());
      setTags([...tags, newTag.trim()]);
      setNewTag('');
      queryClient.invalidateQueries({ queryKey: ['files'] });
      message.success('标签已添加');
    } catch (err: any) {
      message.error('添加失败: ' + err.message);
    }
  };

  const handleRemove = async (tagName: string) => {
    if (!file) return;
    try {
      await tagApi.unbind(file.id, tagName);
      setTags(tags.filter((t) => t !== tagName));
      queryClient.invalidateQueries({ queryKey: ['files'] });
      message.success('标签已移除');
    } catch (err: any) {
      message.error('移除失败: ' + err.message);
    }
  };

  return (
    <Modal
      title={`标签管理 - ${file?.fileName || ''}`}
      open={open}
      onCancel={onClose}
      footer={null}
    >
      <div style={{ marginBottom: 16 }}>
        <Space wrap>
          {tags.map((t) => (
            <Tag key={t} closable onClose={() => handleRemove(t)}>
              {t}
            </Tag>
          ))}
        </Space>
      </div>
      <Space>
        <Input
          value={newTag}
          onChange={(e) => setNewTag(e.target.value)}
          placeholder="输入标签名"
          onPressEnter={handleAdd}
        />
        <Button icon={<PlusOutlined />} onClick={handleAdd}>添加</Button>
      </Space>
    </Modal>
  );
}
