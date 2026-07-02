import apiClient from './client';
import type {
  ApiResponse,
  OkfBundle,
  OkfBrokenLinksPage,
  OkfBundleStats,
  OkfGraphResult,
  OkfIndexRegenResult,
  OkfNodesResult,
  OkfScanReport,
  OkfSearchPage,
  OkfSearchRequest,
  OkfSubgraphRequest,
  OkfTypeCount,
} from './types';

// apiClient's response interceptor already unwraps the axios response — what
// remains is the ApiResponse<T> envelope ({ code, message, data }). Peel one
// more layer so callers receive T directly.
function unwrap<T>(p: Promise<ApiResponse<T>>): Promise<T> {
  return p.then((r) => r.data);
}

export const okfApi = {
  // Bundle lifecycle
  listBundles: () =>
    unwrap<OkfBundle[]>(apiClient.get('/v1/disk/okf/bundles')),
  getBundle: (id: number) =>
    unwrap<OkfBundle>(apiClient.get(`/v1/disk/okf/bundles/${id}`)),
  registerBundle: (publicDirectoryId: number) =>
    unwrap<OkfBundle>(
      apiClient.post('/v1/disk/okf/bundles/register', { publicDirectoryId }),
    ),
  refreshBundle: (id: number) =>
    unwrap<OkfBundle>(apiClient.post(`/v1/disk/okf/bundles/${id}/refresh`)),
  unregisterBundle: (id: number) =>
    unwrap<unknown>(apiClient.delete(`/v1/disk/okf/bundles/${id}`)),

  // Nodes
  listNodes: (bundleId: number, type?: string, tag?: string) =>
    unwrap<OkfNodesResult>(
      apiClient.get(`/v1/disk/okf/bundles/${bundleId}/nodes`, {
        params: { type, tag },
      }),
    ),
  aggregateTypes: () =>
    unwrap<OkfTypeCount[]>(apiClient.get('/v1/disk/okf/types')),
  search: (req: OkfSearchRequest) =>
    unwrap<OkfSearchPage>(apiClient.post('/v1/disk/okf/search', req)),

  // Graph
  neighbors: (
    nodeId: number,
    dir: 'out' | 'in' | 'both' = 'out',
    type?: string,
  ) =>
    unwrap<OkfGraphResult>(
      apiClient.get(`/v1/disk/okf/nodes/${nodeId}/neighbors`, {
        params: { dir, type },
      }),
    ),
  subgraph: (req: OkfSubgraphRequest) =>
    unwrap<OkfGraphResult>(apiClient.post('/v1/disk/okf/subgraph', req)),
  stats: (bundleId: number) =>
    unwrap<OkfBundleStats>(
      apiClient.get(`/v1/disk/okf/bundles/${bundleId}/stats`),
    ),

  // Maintenance
  scanBundle: (bundleId: number) =>
    unwrap<OkfScanReport>(
      apiClient.post(`/v1/disk/okf/bundles/${bundleId}/scan`),
    ),
  listBrokenLinks: (bundleId: number, cursor = 0, limit = 50) =>
    unwrap<OkfBrokenLinksPage>(
      apiClient.get(`/v1/disk/okf/bundles/${bundleId}/broken-links`, {
        params: { cursor, limit },
      }),
    ),
  regenerateIndex: (bundleId: number) =>
    unwrap<OkfIndexRegenResult>(
      apiClient.post(`/v1/disk/okf/bundles/${bundleId}/regenerate-index`),
    ),
};
