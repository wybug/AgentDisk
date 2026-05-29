import axios from 'axios';

const adminClient = axios.create({
  timeout: 30000,
});

adminClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('admin_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

adminClient.interceptors.response.use(
  (response) => {
    const data = response.data;
    if (data.code !== undefined && data.code !== 0) {
      return Promise.reject(new Error(data.message || '请求失败'));
    }
    return data;
  },
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('admin_token');
      window.location.href = '/admin/login';
      return Promise.reject(error);
    }
    const msg = error.response?.data?.message || error.message;
    return Promise.reject(new Error(msg));
  }
);

// Public MFA client — no 401 redirect (MFA endpoints are public, 401 means verification failed)
const mfaPublicClient = axios.create({
  timeout: 30000,
});

mfaPublicClient.interceptors.response.use(
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
  }
);

export const adminApi = {
  login: (username: string, password: string) =>
    adminClient.post('/v1/disk/admin/login', { username, password }),

  checkInitStatus: () =>
    adminClient.get('/v1/disk/admin/init-status'),

  bootstrap: (data: { username: string; password: string; displayName?: string }) =>
    adminClient.post('/v1/disk/admin/bootstrap', data),

  dashboard: () =>
    adminClient.get('/v1/disk/admin/dashboard'),

  // Users
  listUsers: () =>
    adminClient.get('/v1/disk/admin/users'),
  createUser: (data: { username: string; password: string; role?: string; displayName?: string }) =>
    adminClient.post('/v1/disk/admin/users', data),
  changePassword: (username: string, password: string) =>
    adminClient.put(`/v1/disk/admin/users/${username}/password`, { password }),
  deleteUser: (username: string) =>
    adminClient.delete(`/v1/disk/admin/users/${username}`),

  // API Keys
  listApiKeys: () =>
    adminClient.get('/v1/disk/admin/api-keys'),
  createApiKey: (data: { name: string; department?: string }) =>
    adminClient.post('/v1/disk/admin/api-keys', data),
  renameApiKey: (id: number, data: { name: string }) =>
    adminClient.put(`/v1/disk/admin/api-keys/${id}`, data),
  revokeApiKey: (id: number) =>
    adminClient.delete(`/v1/disk/admin/api-keys/${id}`),

  // Public Directories
  listPublicDirectories: () =>
    adminClient.get('/v1/disk/admin/public-directories'),
  createPublicDirectory: (data: { displayName: string; scope: string; department?: string }) =>
    adminClient.post('/v1/disk/admin/public-directories', data),
  updatePublicDirectory: (id: number, data: { displayName?: string; isActive?: boolean }) =>
    adminClient.put(`/v1/disk/admin/public-directories/${id}`, data),
  deletePublicDirectory: (id: number) =>
    adminClient.delete(`/v1/disk/admin/public-directories/${id}`),

  // OAuth2 Config
  getOAuth2Config: () =>
    adminClient.get('/v1/disk/admin/oauth2'),
  updateOAuth2Config: (data: Record<string, unknown>) =>
    adminClient.put('/v1/disk/admin/oauth2', data),
  testOAuth2Config: () =>
    adminClient.post('/v1/disk/admin/oauth2/test'),

  // MFA / WebAuthn
  getMFAStatus: () =>
    adminClient.get('/v1/disk/admin/mfa/status'),
  setMFAEnabled: (enabled: boolean) =>
    adminClient.put('/v1/disk/admin/mfa/enabled', { enabled }),
  beginRegistration: () =>
    adminClient.post('/v1/disk/admin/mfa/registration/begin'),
  finishRegistration: (sessionKey: string, credential: string, name?: string) =>
    adminClient.post('/v1/disk/admin/mfa/registration/finish', { sessionKey, credential, name }),
  listPasskeys: () =>
    adminClient.get('/v1/disk/admin/mfa/credentials'),
  deletePasskey: (id: number) =>
    adminClient.delete(`/v1/disk/admin/mfa/credentials/${id}`),
  renamePasskey: (id: number, name: string) =>
    adminClient.put(`/v1/disk/admin/mfa/credentials/${id}`, { name }),
  beginMFALogin: (sessionToken: string) =>
    mfaPublicClient.post('/v1/disk/admin/mfa/login/begin', { sessionToken }),
  finishMFALogin: (sessionKey: string, credential: string) =>
    mfaPublicClient.post('/v1/disk/admin/mfa/login/finish', { sessionKey, credential }),
};
