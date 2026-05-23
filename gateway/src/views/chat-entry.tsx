import React, { useState, useCallback, useRef, useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { MessageInput } from '@nexus-ai/agent-chat-ui';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Copy, Check, BarChart3, Download, FileText, FileCode, FileImage, File as FileIcon, Eye, EyeOff } from 'lucide-react';

interface Metrics {
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  timeToFirstToken: number;
  duration: number;
}

interface FileArtifact {
  filename: string;
  download_url: string;
  share_code?: string;
  size_bytes: number;
  description?: string;
}

interface AgentOption {
  agentId: string;
  agentName: string;
  agentGroupId: string;
}

interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  metrics?: Metrics;
  fileArtifact?: FileArtifact;
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = async (e: React.MouseEvent) => {
    e.stopPropagation();
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };
  return copied ? (
    <Check size={14} style={{ color: '#22c55e', cursor: 'pointer' }} />
  ) : (
    <Copy size={14} style={{ color: '#999', cursor: 'pointer' }} onClick={handleCopy} />
  );
}

const METRIC_ROWS = [
  { key: 'inputTokens', label: '输入 Token', unit: '' },
  { key: 'outputTokens', label: '输出 Token', unit: '' },
  { key: 'totalTokens', label: '总 Token', unit: '' },
  { key: 'timeToFirstToken', label: '首 Token 延迟', unit: ' s' },
  { key: 'duration', label: '运行耗时', unit: ' s' },
] as const;

function MetricsPopover({ metrics }: { metrics: Metrics }) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top: number; left: number }>({ top: 0, left: 0 });
  const iconRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (iconRef.current && !iconRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  const handleToggle = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!open && iconRef.current) {
      const r = iconRef.current.getBoundingClientRect();
      const pw = 240, ph = 130;
      const top = Math.min(Math.max(r.top - ph - 6, 8), window.innerHeight - ph - 8);
      const left = Math.min(Math.max(r.left, 8), window.innerWidth - pw - 8);
      setPos({ top, left });
    }
    setOpen(v => !v);
  };

  const fmt = (key: string, val: number) =>
    key.includes('Token') ? val.toLocaleString() : val.toFixed(3);

  return (
    <div ref={iconRef} style={{ position: 'relative', display: 'inline-flex' }}>
      <BarChart3 size={14} style={{ color: '#999', cursor: 'pointer' }} onClick={handleToggle} />
      {open && (
        <div style={{ ...styles.popover, top: pos.top, left: pos.left }}>
          {METRIC_ROWS.map(r => (
            <div key={r.key} style={styles.popoverRow}>
              <span>{r.label}</span>
              <span style={styles.popoverValue}>{fmt(r.key, metrics[r.key])}{r.unit}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

function fileIcon(filename: string) {
  const ext = filename.split('.').pop()?.toLowerCase() ?? '';
  if (['md', 'txt', 'rtf'].includes(ext)) return <FileText size={28} style={{ color: '#667eea', flexShrink: 0 }} />;
  if (['js', 'ts', 'py', 'go', 'java', 'html', 'css', 'json', 'yaml', 'yml', 'sh'].includes(ext)) return <FileCode size={28} style={{ color: '#667eea', flexShrink: 0 }} />;
  if (['png', 'jpg', 'jpeg', 'gif', 'svg', 'webp'].includes(ext)) return <FileImage size={28} style={{ color: '#667eea', flexShrink: 0 }} />;
  return <FileIcon size={28} style={{ color: '#667eea', flexShrink: 0 }} />;
}

function FileArtifactCard({ artifact }: { artifact: FileArtifact }) {
  const [copied, setCopied] = useState(false);

  const handleCopyShareCode = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!artifact.share_code) return;
    await navigator.clipboard.writeText(artifact.share_code);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <div style={styles.artifactCard}>
      <div style={styles.artifactLeft}>
        {fileIcon(artifact.filename)}
        <div style={styles.artifactInfo}>
          <div style={styles.artifactName}>{artifact.filename}</div>
          <div style={styles.artifactMeta}>
            {formatSize(artifact.size_bytes)}{artifact.description ? ` · ${artifact.description}` : ''}
          </div>
        </div>
      </div>
      <div style={styles.artifactActions}>
        {artifact.download_url && (
          <a href={artifact.download_url} target="_blank" rel="noopener noreferrer" style={styles.artifactBtn} title="下载">
            <Download size={14} />
            <span>下载</span>
          </a>
        )}
        {artifact.share_code && (
          <button onClick={handleCopyShareCode} style={styles.artifactBtn} title="复制分享码">
            {copied ? <Check size={14} style={{ color: '#22c55e' }} /> : <Copy size={14} />}
            <span>{copied ? '已复制' : '分享码'}</span>
          </button>
        )}
      </div>
    </div>
  );
}

function MessageBubble({ msg }: { msg: Message }) {
  const isUser = msg.role === 'user';
  const [showMarkdown, setShowMarkdown] = useState(true);

  return (
    <div style={{ ...styles.bubbleRow, justifyContent: isUser ? 'flex-end' : 'flex-start' }}>
      <div style={{ ...styles.bubble, ...(isUser ? styles.userBubble : styles.assistantBubble) }}>
        {isUser ? (
          <div style={styles.bubbleContent}>{msg.content}</div>
        ) : (
          <div className="markdown-body" style={styles.bubbleContent}>
            {msg.content ? (
              showMarkdown ? (
                <Markdown remarkPlugins={[remarkGfm]}>{msg.content}</Markdown>
              ) : (
                <pre style={styles.rawText}>{msg.content}</pre>
              )
            ) : (
              <span style={{ color: '#aaa' }}>...</span>
            )}
            {msg.fileArtifact && <FileArtifactCard artifact={msg.fileArtifact} />}
          </div>
        )}
        {!isUser && msg.content && (
          <div style={styles.bubbleFooter}>
            <CopyButton text={msg.content} />
            <span
              style={{ cursor: 'pointer', color: '#999', display: 'inline-flex' }}
              onClick={() => setShowMarkdown(v => !v)}
              title={showMarkdown ? '显示原始文本' : '显示 Markdown'}
            >
              {showMarkdown ? <EyeOff size={14} /> : <Eye size={14} />}
            </span>
            {msg.metrics && <MetricsPopover metrics={msg.metrics} />}
          </div>
        )}
      </div>
    </div>
  );
}

function ChatApp() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [agents, setAgents] = useState<AgentOption[]>([]);
  const [selectedAgent, setSelectedAgent] = useState('');
  const abortRef = useRef<AbortController | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    fetch('/api/agents').then(r => r.json()).then((data: AgentOption[]) => setAgents(data)).catch(() => {});
  }, []);

  const scrollToBottom = () => {
    requestAnimationFrame(() => {
      if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    });
  };

  const sendMessage = useCallback(async (text: string) => {
    if (!text.trim() || loading) return;

    const userMsg: Message = { id: crypto.randomUUID(), role: 'user', content: text };
    setMessages(prev => [...prev, userMsg]);
    setInput('');
    setLoading(true);
    scrollToBottom();

    const assistantId = crypto.randomUUID();
    setMessages(prev => [...prev, { id: assistantId, role: 'assistant', content: '' }]);
    scrollToBottom();

    const controller = new AbortController();
    abortRef.current = controller;

    try {
      const resp = await fetch('/process', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ input: [{ role: 'user', content: [{ type: 'text', text }] }], agentId: selectedAgent || undefined }),
        signal: controller.signal,
      });

      if (!resp.ok || !resp.body) throw new Error(`请求失败: ${resp.status}`);

      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      let accumulated = '';
      let metrics: Metrics | undefined;
      let fileArtifact: FileArtifact | undefined;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        const chunk = decoder.decode(value, { stream: true });
        const lines = chunk.split('\n');

        for (const line of lines) {
          if (!line.startsWith('data: ')) continue;
          const data = line.slice(6).trim();
          if (!data) continue;
          try {
            const parsed = JSON.parse(data);
            if (parsed.error) {
              accumulated = parsed.error;
            } else if (parsed.object === 'content' && parsed.delta && parsed.text) {
              accumulated += parsed.text;
            } else if (parsed.object === 'content' && parsed.status === 'completed' && parsed.text && !accumulated) {
              accumulated = parsed.text;
            } else if (parsed.object === 'response' && parsed.status === 'completed' && parsed.usage) {
              const u = parsed.usage;
              metrics = {
                inputTokens: u.input_tokens ?? 0,
                outputTokens: u.output_tokens ?? 0,
                totalTokens: u.total_tokens ?? 0,
                timeToFirstToken: u.time_to_first_token ?? 0,
                duration: u.duration ?? 0,
              };
              if (parsed.metadata?.file_artifact) {
                fileArtifact = parsed.metadata.file_artifact;
              }
            }
          } catch { /* skip */ }
          setMessages(prev =>
            prev.map(m => m.id === assistantId ? { ...m, content: accumulated, metrics, fileArtifact } : m)
          );
          scrollToBottom();
        }
      }
    } catch (err: any) {
      if (err.name !== 'AbortError') {
        setMessages(prev =>
          prev.map(m => m.id === assistantId ? { ...m, content: `错误: ${err.message}` } : m)
        );
      }
    } finally {
      setLoading(false);
      abortRef.current = null;
    }
  }, [loading]);

  const handleAbort = useCallback(() => { abortRef.current?.abort(); }, []);

  return (
    <div style={styles.container}>
      {agents.length > 0 && (
        <div style={styles.agentBar}>
          <span style={styles.agentLabel}>身份：</span>
          <select
            value={selectedAgent}
            onChange={e => setSelectedAgent(e.target.value)}
            style={styles.agentSelect}
          >
            <option value="">用户身份</option>
            {agents.map(a => <option key={a.agentId} value={a.agentId}>{a.agentName} ({a.agentId})</option>)}
          </select>
        </div>
      )}
      <div ref={scrollRef} style={styles.messageArea}>
        {messages.map(m => <MessageBubble key={m.id} msg={m} />)}
      </div>
      <MessageInput
        value={input}
        onChange={setInput}
        onSubmit={sendMessage}
        onAbort={handleAbort}
        loading={loading}
        placeholder="输入消息..."
      />
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: { display: 'flex', flexDirection: 'column', height: '100%', background: '#f9fafb' },
  agentBar: { display: 'flex', alignItems: 'center', padding: '8px 16px', background: '#fff', borderBottom: '1px solid #e5e7eb' },
  agentLabel: { fontSize: '13px', color: '#666', marginRight: 8 },
  agentSelect: { padding: '4px 8px', borderRadius: 6, border: '1px solid #d1d5db', fontSize: '13px', background: '#fff' },
  messageArea: { flex: 1, overflow: 'auto', padding: '16px', display: 'flex', flexDirection: 'column', gap: '12px' },
  bubbleRow: { display: 'flex' },
  bubble: { maxWidth: '80%', padding: '10px 14px', borderRadius: '12px', lineHeight: 1.6, fontSize: '14px', wordBreak: 'break-word' as const },
  userBubble: { background: '#667eea', color: '#fff', borderBottomRightRadius: '4px' },
  assistantBubble: { background: '#fff', color: '#333', border: '1px solid #e5e7eb', borderBottomLeftRadius: '4px' },
  bubbleContent: { whiteSpace: 'pre-wrap' as const },
  rawText: { margin: 0, padding: 0, background: 'none', fontSize: 'inherit', lineHeight: 'inherit', whiteSpace: 'pre-wrap' as const, wordBreak: 'break-word' as const, color: 'inherit', fontFamily: 'monospace' },
  bubbleFooter: { display: 'flex', alignItems: 'center', gap: '8px', marginTop: '8px', paddingTop: '8px', borderTop: '1px solid #f0f0f0' },
  popover: {
    position: 'fixed',
    background: '#fff',
    border: '1px solid #e5e7eb',
    borderRadius: '8px',
    padding: '8px 12px',
    boxShadow: '0 4px 12px rgba(0,0,0,0.12)',
    zIndex: 1000,
    fontSize: '12px',
    lineHeight: 1.4,
  },
  popoverRow: { display: 'flex', justifyContent: 'space-between', gap: '16px', padding: '2px 0', color: '#555' },
  popoverValue: { color: '#667eea', fontWeight: 500, whiteSpace: 'nowrap' as const },
  artifactCard: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 12,
    marginTop: 10,
    padding: '10px 12px',
    background: '#f8f9fb',
    border: '1px solid #e5e7eb',
    borderRadius: 10,
  },
  artifactLeft: { display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 },
  artifactInfo: { minWidth: 0 },
  artifactName: { fontSize: 13, fontWeight: 500, color: '#333', whiteSpace: 'nowrap' as const, overflow: 'hidden', textOverflow: 'ellipsis' },
  artifactMeta: { fontSize: 11, color: '#999', marginTop: 2 },
  artifactActions: { display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0 },
  artifactBtn: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 4,
    padding: '4px 10px',
    fontSize: 12,
    color: '#667eea',
    background: '#fff',
    border: '1px solid #e0e0e0',
    borderRadius: 6,
    cursor: 'pointer',
    textDecoration: 'none',
  } as React.CSSProperties,
};

const container = document.getElementById('chat-root');
if (container) createRoot(container).render(<ChatApp />);
