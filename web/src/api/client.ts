import axios from 'axios';

const apiClient = axios.create({
  withCredentials: true,
  timeout: 30000,
});

apiClient.interceptors.response.use(
  (response) => {
    const data = response.data;
    if (data.code !== undefined && data.code !== 0) {
      return Promise.reject(new Error(data.message || '请求失败'));
    }
    return data;
  },
  (error) => {
    if (error.response?.status === 401) {
      window.location.href = '/auth/login';
      return Promise.reject(error);
    }
    const msg = error.response?.data?.message || error.message;
    return Promise.reject(new Error(msg));
  }
);

export default apiClient;
