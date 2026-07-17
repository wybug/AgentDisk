import { Modal, Spin } from 'antd';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { useQuery } from '@tanstack/react-query';
import { okfShareApi } from '@/api/okf';

// safeUrlTransform allowlists URL schemes rendered by the share markdown view.
// react-markdown does not render raw HTML (so <script> is escaped), but writer-
// supplied markdown could still ship executable URLs like [x](javascript:...).
// Allow relative links, anchors, absolute paths, and a small safe-protocol set;
// drop everything else (javascript:, data:, vbscript:, file:, ...) so a shared
// bundle cannot surface dangerous links to anonymous recipients.
function safeUrlTransform(url: string): string {
  if (url.startsWith('#') || url.startsWith('/') || url.startsWith('.')) return url;
  const colon = url.indexOf(':');
  if (colon === -1) return url; // schemeless / relative
  const scheme = url.slice(0, colon).toLowerCase();
  if (scheme === 'http' || scheme === 'https' || scheme === 'mailto' || scheme === 'tel') return url;
  return '';
}

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
            urlTransform={safeUrlTransform}
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
