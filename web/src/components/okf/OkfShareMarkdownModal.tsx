import { Modal, Spin } from 'antd';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { useQuery } from '@tanstack/react-query';
import { okfShareApi } from '@/api/okf';

interface Props {
  code: string;
  nodeId: number | null;
  extractCode?: string;
  onClose: () => void;
}

// OkfShareMarkdownModal renders the markdown body of a node inline (no
// /preview/:fileId redirect — that route is authed, the share recipient has
// no session). Body is fetched from the public getNode endpoint which
// returns the raw markdown bytes alongside the frontmatter.
export default function OkfShareMarkdownModal({ code, nodeId, extractCode, onClose }: Props) {
  const open = nodeId !== null;
  const { data, isFetching, error } = useQuery({
    queryKey: ['share-node-markdown', code, nodeId],
    queryFn: () => okfShareApi.getNode(code, nodeId!, extractCode),
    enabled: open,
  });

  return (
    <Modal
      title={data?.title || '节点内容'}
      open={open}
      onCancel={onClose}
      footer={null}
      width={800}
      destroyOnClose
    >
      {isFetching && (
        <div style={{ textAlign: 'center', padding: 24 }}>
          <Spin />
        </div>
      )}
      {error && <div style={{ color: '#ff4d4f' }}>{String(error.message)}</div>}
      {data?.markdown && (
        <div className="markdown-body" style={{ maxHeight: '70vh', overflow: 'auto' }}>
          <ReactMarkdown
            remarkPlugins={[remarkGfm]}
            components={{
              code({ className, children, ...rest }) {
                const match = /language-(\w+)/.exec(className || '');
                const isInline = !className;
                return isInline ? (
                  <code className={className} {...rest}>
                    {children}
                  </code>
                ) : (
                  <SyntaxHighlighter
                    language={match?.[1] || 'text'}
                    style={oneDark}
                    PreTag="div"
                  >
                    {String(children).replace(/\n$/, '')}
                  </SyntaxHighlighter>
                );
              },
            }}
          >
            {data.markdown}
          </ReactMarkdown>
        </div>
      )}
    </Modal>
  );
}
