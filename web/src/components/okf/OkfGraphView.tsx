import { useEffect, useRef, useState } from 'react';
import { Button, InputNumber, Select, Space, Spin, Typography, App } from 'antd';
import cytoscape, { type Core } from 'cytoscape';
import coseBilkent from 'cytoscape-cose-bilkent';
import { ApartmentOutlined, ClearOutlined, ReloadOutlined } from '@ant-design/icons';
import { okfApi } from '@/api/okf';
import type { OkfEdge, OkfNode, OkfTypeCount } from '@/api/types';

// cytoscape-cose-bilkent registers itself on the cytoscape singleton. The
// registration is idempotent so a module-level guard is enough.
let layoutRegistered = false;
function ensureLayout() {
  if (layoutRegistered) return;
  // useCoseBilkent may throw when called twice; the guard makes that a no-op.
  try {
    cytoscape.use(coseBilkent as unknown as cytoscape.Ext);
  } catch {
    // already registered (e.g. HMR)
  }
  layoutRegistered = true;
}

// 10-color palette keyed off the type string. We hash so the same type maps
// to the same color across renders / page loads.
const PALETTE = [
  '#5B8FF9', '#5AD8A6', '#5D7092', '#F6BD16', '#E8684A',
  '#6DC8EC', '#9270CA', '#FF9D4D', '#269A99', '#FF99C3',
];
function colorForType(type: string): string {
  let h = 0;
  for (let i = 0; i < type.length; i++) h = (h * 31 + type.charCodeAt(i)) | 0;
  return PALETTE[Math.abs(h) % PALETTE.length];
}

function truncate(s: string, n: number): string {
  return s.length > n ? `${s.slice(0, n)}…` : s;
}

// In the authed path these default to okfApi. The share path passes
// share-mode fetchers that route through /v1/disk/share/:code/... instead.
export interface OkfGraphViewFetchers {
  subgraph: (req: {
    bundleId: number;
    types?: string[];
    maxNodes?: number;
  }) => Promise<{ nodes: OkfNode[]; edges: OkfEdge[] }>;
  neighbors: (nodeId: number) => Promise<{ nodes: OkfNode[]; edges: OkfEdge[] }>;
  aggregateTypes?: () => Promise<OkfTypeCount[]>;
}

interface Props {
  bundleId: number;
  onNodeDoubleClick: (node: OkfNode) => void;
  fetchers?: OkfGraphViewFetchers;
}

// OkfGraphView mounts a Cytoscape instance, loads the bundle's subgraph on
// first render, and expands the 1-hop neighborhood when the user taps a
// node. Double-tap opens the frontmatter drawer (delegated to the host
// page). The component owns the cytoscape instance and tears it down on
// unmount; the React-side nodes/edges maps are the source of truth that
// get re-applied whenever we add nodes or edges.
export default function OkfGraphView({ bundleId, onNodeDoubleClick, fetchers }: Props) {
  const subgraphFn = fetchers?.subgraph ?? okfApi.subgraph;
  const neighborsFn = async (nodeId: number) => {
    if (fetchers?.neighbors) return fetchers.neighbors(nodeId);
    const r = await okfApi.neighbors(nodeId);
    return { nodes: r.nodes || [], edges: r.edges || [] };
  };
  const aggregateTypesFn = fetchers?.aggregateTypes;
  const containerRef = useRef<HTMLDivElement | null>(null);
  const cyRef = useRef<Core | null>(null);
  // Keep OkfNode by id so double-tap can hand the caller the full record.
  const nodesMap = useRef<Map<number, OkfNode>>(new Map());
  const [types, setTypes] = useState<OkfTypeCount[]>([]);
  const [typeFilter, setTypeFilter] = useState<string[] | undefined>(undefined);
  const [maxNodes, setMaxNodes] = useState(200);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<string>('');
  // Distinct types currently on the canvas — drives the color legend. Updated
  // as nodes are merged in (works in both authed and share mode, unlike the
  // aggregate-types fetch which is authed-only).
  const [legendTypes, setLegendTypes] = useState<string[]>([]);
  const { message } = App.useApp();

  // Apply current React state (nodes/edges in the cytoscape instance) and
  // run a fresh layout. We rebuild the elements from the live cytoscape
  // instance so the layout sees everything currently on the canvas.
  const runLayout = () => {
    const cy = cyRef.current;
    if (!cy) return;
    cy.layout({
      name: 'cose-bilkent',
      animate: false,
      idealEdgeLength: 100,
      nodeRepulsion: 8000,
      fit: true,
      padding: 30,
    } as cytoscape.LayoutOptions).run();
  };

  // Merge new nodes/edges into the cytoscape instance + our shadow map.
  // Existing ids are skipped so re-fetching a neighborhood is a no-op.
  const merge = (nodes: OkfNode[], edges: OkfEdge[]) => {
    const cy = cyRef.current;
    if (!cy) return;
    const newNodeIds = new Set<number>();
    for (const n of nodes) {
      nodesMap.current.set(n.nodeId, n);
      if (!cy.hasElementWithId(`n${n.nodeId}`)) {
        newNodeIds.add(n.nodeId);
        cy.add({
          group: 'nodes',
          data: {
            id: `n${n.nodeId}`,
            label: truncate(n.title || n.relPath, 20),
            color: colorForType(n.type),
            broken: n.hasBrokenLink ? 'true' : 'false',
          },
        });
      }
    }
    for (const e of edges) {
      const id = `e${e.edgeId}`;
      if (cy.hasElementWithId(id)) continue;
      // Skip edges that point at a node we don't (yet) have on the canvas.
      // This happens when the bundle has edges to nodes outside the loaded
      // subgraph; rendering a half-edge looks broken.
      if (!cy.hasElementWithId(`n${e.dstNodeId}`)) continue;
      cy.add({
        group: 'edges',
        data: {
          id,
          source: `n${e.srcNodeId}`,
          target: `n${e.dstNodeId}`,
          label: truncate(e.linkText || '', 16),
          broken: e.dstExists ? 'false' : 'true',
        },
      });
    }
    setLegendTypes((prev) => {
      const set = new Set(prev);
      for (const n of nodes) if (n.type) set.add(n.type);
      return Array.from(set).sort();
    });
    runLayout();
  };

  const loadSubgraph = async () => {
    setLoading(true);
    setStatus('加载整图…');
    try {
      const res = await subgraphFn({
        bundleId,
        types: typeFilter,
        maxNodes,
      });
      // Reset the canvas before applying — "load" replaces, "expand" merges.
      cyRef.current?.elements().remove();
      nodesMap.current.clear();
      setLegendTypes([]);
      merge(res.nodes || [], res.edges || []);
      setStatus(`已加载 ${res.nodes?.length ?? 0} 个节点 / ${res.edges?.length ?? 0} 条边`);
    } catch {
      message.error('加载图谱失败');
      setStatus('');
    } finally {
      setLoading(false);
    }
  };

  const expandNeighbors = async (nodeId: number) => {
    setStatus(`加载邻居 #${nodeId}…`);
    try {
      const res = await neighborsFn(nodeId);
      merge(res.nodes || [], res.edges || []);
      setStatus(`邻居已展开：+${res.nodes?.length ?? 0} 节点`);
    } catch {
      message.error('加载邻居失败');
      setStatus('');
    }
  };

  const handleClear = () => {
    cyRef.current?.elements().remove();
    nodesMap.current.clear();
    setLegendTypes([]);
    setStatus('已清空');
  };

  // Init cytoscape once.
  useEffect(() => {
    ensureLayout();
    if (!containerRef.current) return;
    const cy = cytoscape({
      container: containerRef.current,
      style: [
        {
          selector: 'node',
          style: {
            label: 'data(label)',
            'background-color': 'data(color)',
            color: '#333',
            'font-size': 11,
            width: 36,
            height: 36,
            'text-valign': 'bottom',
            'text-halign': 'center',
            'text-margin-y': 4,
          },
        },
        {
          selector: 'node[broken="true"]',
          style: { 'border-color': '#ff4d4f', 'border-width': 3 },
        },
        {
          selector: 'edge',
          style: {
            width: 2,
            'line-color': '#888',
            'target-arrow-color': '#888',
            'target-arrow-shape': 'triangle',
            'curve-style': 'bezier',
            // Render the link text on the edge so users see *why* two nodes
            // connect, not just that they do. Autorotate keeps the label along
            // the edge; the white pill keeps it legible over crossings.
            label: 'data(label)',
            'font-size': 9,
            color: '#666',
            'text-rotation': 'autorotate',
            'text-background-color': '#fff',
            'text-background-opacity': 0.85,
            'text-background-padding': '1',
            'text-background-shape': 'roundrectangle',
          },
        },
        {
          selector: 'edge[broken="true"]',
          style: { 'line-color': '#ff4d4f', 'line-style': 'dashed', 'target-arrow-color': '#ff4d4f' },
        },
      ],
      minZoom: 0.2,
      maxZoom: 3,
    });
    cy.on('tap', 'node', (evt) => {
      const id = Number(evt.target.id().replace(/^n/, ''));
      if (Number.isFinite(id)) void expandNeighbors(id);
    });
    cy.on('dbltap', 'node', (evt) => {
      const id = Number(evt.target.id().replace(/^n/, ''));
      const node = nodesMap.current.get(id);
      if (node) onNodeDoubleClick(node);
    });
    cyRef.current = cy;
    const nodesMapAtMount = nodesMap.current;
    return () => {
      cy.destroy();
      cyRef.current = null;
      nodesMapAtMount.clear();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Load type list for the filter dropdown once. Skipped in share mode
  // (no public aggregateTypes endpoint) — the dropdown is hidden instead.
  useEffect(() => {
    if (!aggregateTypesFn) {
      return;
    }
    let cancelled = false;
    aggregateTypesFn()
      .then((t) => {
        if (!cancelled) setTypes(t);
      })
      .catch(() => {
        if (!cancelled) setTypes([]);
      });
    return () => {
      cancelled = true;
    };
  }, [aggregateTypesFn]);

  // Initial subgraph load.
  useEffect(() => {
    // Defer the first setState past the synchronous effect body so we don't
    // trigger cascading renders (react-hooks/set-state-in-effect).
    const t = setTimeout(() => {
      void loadSubgraph();
    }, 0);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div>
      <Space style={{ marginBottom: 12 }} wrap>
        {aggregateTypesFn && (
          <Select
            mode="multiple"
            allowClear
            placeholder="按 type 过滤（留空为全部）"
            style={{ minWidth: 240 }}
            value={typeFilter}
            onChange={(v) => setTypeFilter(v)}
            options={types.map((t) => ({ value: t.type, label: `${t.type} (${t.count})` }))}
          />
        )}
        <Space.Compact>
          <span style={{ alignSelf: 'center', marginRight: 6 }}>maxNodes</span>
          <InputNumber
            min={10}
            max={500}
            step={50}
            value={maxNodes}
            onChange={(v) => setMaxNodes(typeof v === 'number' ? v : 200)}
            style={{ width: 90 }}
          />
        </Space.Compact>
        <Button type="primary" icon={<ApartmentOutlined />} loading={loading} onClick={loadSubgraph}>
          加载整图
        </Button>
        <Button icon={<ClearOutlined />} onClick={handleClear}>
          清空
        </Button>
        <Button icon={<ReloadOutlined />} onClick={loadSubgraph}>
          重新布局
        </Button>
        <Typography.Text type="secondary">{status}</Typography.Text>
      </Space>
      <div style={{ position: 'relative', height: 600, border: '1px solid #d9d9d9', borderRadius: 6, overflow: 'hidden', background: '#fafafa' }}>
        <div ref={containerRef} style={{ width: '100%', height: '100%' }} />
        {legendTypes.length > 0 && (
          <div
            style={{
              position: 'absolute',
              top: 8,
              right: 8,
              background: 'rgba(255,255,255,0.9)',
              border: '1px solid #e8e8e8',
              borderRadius: 4,
              padding: '4px 8px',
              fontSize: 11,
              lineHeight: '20px',
              maxWidth: 260,
              pointerEvents: 'none',
              zIndex: 1,
            }}
          >
            {legendTypes.map((t) => (
              <span key={t} style={{ display: 'inline-flex', alignItems: 'center', marginRight: 8 }}>
                <span
                  style={{
                    display: 'inline-block',
                    width: 10,
                    height: 10,
                    borderRadius: 2,
                    background: colorForType(t),
                    marginRight: 4,
                  }}
                />
                {t}
              </span>
            ))}
          </div>
        )}
        {loading && (
          <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(255,255,255,0.4)' }}>
            <Spin />
          </div>
        )}
      </div>
      <Typography.Text type="secondary" style={{ fontSize: 12, marginTop: 8, display: 'block' }}>
        单击节点：加载该节点的 1 跳邻居。双击节点：查看 frontmatter。鼠标滚轮：缩放。
      </Typography.Text>
    </div>
  );
}
