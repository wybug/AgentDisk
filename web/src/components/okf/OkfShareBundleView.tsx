import { useMemo, useState } from 'react';
import { Alert, Skeleton, Tabs, Typography } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { okfShareApi } from '@/api/okf';
import type { OkfNode } from '@/api/types';
import OkfNodeList from './OkfNodeList';
import OkfGraphView from './OkfGraphView';
import OkfFrontmatterDrawer from './OkfFrontmatterDrawer';
import OkfShareMarkdownModal from './OkfShareMarkdownModal';

interface Props {
  code: string;
  bundleId: number;
  extractCode?: string;
}

// OkfShareBundleView is the recipient-facing view of a bundle share. It
// renders two read-only tabs (节点 + 图谱) and reuses the same presentational
// components as the authed bundle detail page — only the data fetchers are
// swapped to point at the public /v1/disk/share/:code/... endpoints. No
// refresh / rebuild / unregister buttons: those are admin operations.
export default function OkfShareBundleView({ code, bundleId, extractCode }: Props) {
  const [selectedNode, setSelectedNode] = useState<OkfNode | null>(null);
  const [markdownNodeId, setMarkdownNodeId] = useState<number | null>(null);

  const { data: bundle, isFetching, error } = useQuery({
    queryKey: ['share-bundle', code],
    queryFn: () => okfShareApi.getBundle(code, bundleId, extractCode),
  });

  // Stable fetcher closures so OkfNodeList/OkfGraphView effect deps stay calm.
  const nodeListFetchers = useMemo(
    () => ({
      listNodes: (_bundleId: number, type?: string, tag?: string) =>
        okfShareApi.listNodes(code, bundleId, type, tag, extractCode),
    }),
    [code, bundleId, extractCode],
  );

  const graphFetchers = useMemo(
    () => ({
      subgraph: (req: { bundleId: number; types?: string[]; maxNodes?: number }) =>
        okfShareApi.subgraph(code, { ...req, bundleId }, extractCode),
      neighbors: (nodeId: number) =>
        okfShareApi.neighbors(code, nodeId, 'out', undefined, extractCode),
    }),
    [code, bundleId, extractCode],
  );

  if (isFetching) {
    return <Skeleton active />;
  }
  if (error) {
    return (
      <Alert
        type="error"
        showIcon
        message="加载 Bundle 失败"
        description={String((error as Error).message)}
      />
    );
  }

  return (
    <div>
      <Typography.Title level={3}>{bundle?.title || 'Bundle'}</Typography.Title>
      {bundle?.description && (
        <Typography.Paragraph type="secondary">
          {bundle.description}
        </Typography.Paragraph>
      )}

      <Tabs
        defaultActiveKey="nodes"
        items={[
          {
            key: 'nodes',
            label: '节点',
            children: (
              <OkfNodeList
                bundleId={bundleId}
                onNodeClick={(n) => setSelectedNode(n)}
                fetchers={nodeListFetchers}
              />
            ),
          },
          {
            key: 'graph',
            label: '图谱',
            children: (
              <OkfGraphView
                bundleId={bundleId}
                onNodeDoubleClick={(n) => setSelectedNode(n)}
                fetchers={graphFetchers}
              />
            ),
          },
        ]}
      />

      <OkfFrontmatterDrawer
        node={selectedNode}
        onClose={() => setSelectedNode(null)}
        onPreview={(fileId) => {
          void fileId;
          if (selectedNode) setMarkdownNodeId(selectedNode.nodeId);
        }}
      />

      <OkfShareMarkdownModal
        code={code}
        nodeId={markdownNodeId}
        extractCode={extractCode}
        onClose={() => setMarkdownNodeId(null)}
      />
    </div>
  );
}
