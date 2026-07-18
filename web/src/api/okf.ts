import axios from 'axios';
import apiClient from './client';
import type {
  ApiResponse,
  OkfBundle,
  OkfBrokenLinksPage,
  OkfBundleStats,
  OkfGraphResult,
  OkfIndexRegenResult,
  OkfNode,
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

// Public (no-auth) client for share-code endpoints. Same envelope-unwrap
// behavior as apiClient, but no `withCredentials` and no 401 redirect — the
// share code IS the credential, and a 401 here would be a bug, not a session
// expiry.
const publicClient = axios.create({ timeout: 30000 });

publicClient.interceptors.response.use(
  (response) => {
    const data = response.data;
    if (data.code !== undefined && data.code !== 0) {
      return Promise.reject(new Error(data.message || '请求失败'));
    }
    return data;
  },
  (error) => {
    const msg = error.response?.data?.message || error.message;
    return Promise.reject(new Error(msg));
  },
);

function unwrapPublic<T>(p: Promise<ApiResponse<T>>): Promise<T> {
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
  listNodes: (bundleId: number, type?: string, tag?: string, cursor = 0, limit = 50) =>
    unwrap<OkfNodesResult>(
      apiClient.get(`/v1/disk/okf/bundles/${bundleId}/nodes`, {
        params: { type, tag, cursor: cursor || undefined, limit },
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

// okfShareApi hits the public share-code OKF endpoints (no JWT/API Key).
// extractCode is passed as a query param so the backend can re-verify per
// request; the share code lives in the path.
export const okfShareApi = {
  getBundle: (code: string, bundleId: number, extractCode?: string) =>
    unwrapPublic<OkfBundle>(
      publicClient.get(`/v1/disk/share/${code}/bundle`, {
        params: { bundleId, extractCode },
      }),
    ),

  listNodes: (
    code: string,
    bundleId: number,
    type?: string,
    tag?: string,
    extractCode?: string,
    cursor = 0,
    limit = 50,
  ) =>
    unwrapPublic<OkfNodesResult>(
      publicClient.get(`/v1/disk/share/${code}/nodes`, {
        params: { bundleId, type, tag, extractCode, cursor: cursor || undefined, limit },
      }),
    ),

  subgraph: (
    code: string,
    req: { bundleId: number; types?: string[]; maxNodes?: number },
    extractCode?: string,
  ) =>
    unwrapPublic<OkfGraphResult>(
      publicClient.get(`/v1/disk/share/${code}/subgraph`, {
        params: {
          bundleId: req.bundleId,
          types: req.types?.join(','),
          maxNodes: req.maxNodes,
          extractCode,
        },
      }),
    ),

  neighbors: (
    code: string,
    nodeId: number,
    dir: 'out' | 'in' | 'both' = 'out',
    type?: string,
    extractCode?: string,
  ) =>
    unwrapPublic<OkfGraphResult>(
      publicClient.get(
        `/v1/disk/share/${code}/nodes/${nodeId}/neighbors`,
        { params: { dir, type, extractCode } },
      ),
    ),

  getNode: (code: string, nodeId: number, extractCode?: string) =>
    unwrapPublic<OkfNode & { markdown: string }>(
      publicClient.get(`/v1/disk/share/${code}/nodes/${nodeId}`, {
        params: { extractCode },
      }),
    ),
};
