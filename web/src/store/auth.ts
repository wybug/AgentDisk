import { create } from 'zustand';
import { spaceApi } from '@/api/space';
import axios from 'axios';

interface AuthState {
  isAuthenticated: boolean;
  isLoading: boolean;
  userName: string | null;
  userId: string | null;
  department: string | null;
  checkAuth: () => Promise<void>;
  logout: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set) => ({
  isAuthenticated: false,
  isLoading: true,
  userName: null,
  userId: null,
  department: null,

  checkAuth: async () => {
    try {
      const space = await spaceApi.get();
      set({
        isAuthenticated: true,
        isLoading: false,
        userId: space.userId,
      });
    } catch {
      set({ isAuthenticated: false, isLoading: false });
    }
  },

  logout: async () => {
    try {
      await fetch('/auth/logout', { method: 'POST', credentials: 'include' });
    } catch {
      // ignore
    }
    set({ isAuthenticated: false, userName: null, userId: null, department: null });
    try {
      const res = await axios.get('/auth/status');
      if (res.data?.data?.oauth2 === false) {
        window.location.href = '/auth/unavailable';
      } else {
        window.location.href = '/auth/login';
      }
    } catch {
      window.location.href = '/auth/unavailable';
    }
  },
}));
