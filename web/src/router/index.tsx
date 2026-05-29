import { createBrowserRouter, Navigate } from 'react-router-dom';
import { lazy, Suspense } from 'react';
import { Spin } from 'antd';
import ProtectedRoute from './ProtectedRoute';
import AppLayout from '@/components/layout/AppLayout';

// eslint-disable-next-line react-refresh/only-export-components
const ExplorerPage = lazy(() => import('@/pages/ExplorerPage'));
// eslint-disable-next-line react-refresh/only-export-components
const RecycleBinPage = lazy(() => import('@/pages/RecycleBinPage'));
// eslint-disable-next-line react-refresh/only-export-components
const SharesPage = lazy(() => import('@/pages/SharesPage'));
// eslint-disable-next-line react-refresh/only-export-components
const TagsPage = lazy(() => import('@/pages/TagsPage'));
// eslint-disable-next-line react-refresh/only-export-components
const PermissionsPage = lazy(() => import('@/pages/PermissionsPage'));
// eslint-disable-next-line react-refresh/only-export-components
const ShareAccessPage = lazy(() => import('@/pages/ShareAccessPage'));
// eslint-disable-next-line react-refresh/only-export-components
const PreviewPage = lazy(() => import('@/pages/PreviewPage'));
// eslint-disable-next-line react-refresh/only-export-components
const PublicDirectoriesPage = lazy(() => import('@/pages/PublicDirectoriesPage'));
// eslint-disable-next-line react-refresh/only-export-components
const AdminLoginPage = lazy(() => import('@/pages/AdminLoginPage'));
// eslint-disable-next-line react-refresh/only-export-components
const AdminSetupPage = lazy(() => import('@/pages/AdminSetupPage'));
// eslint-disable-next-line react-refresh/only-export-components
const AdminPage = lazy(() => import('@/pages/AdminPage'));
// eslint-disable-next-line react-refresh/only-export-components
const PublicDirManager = lazy(() => import('@/components/admin/PublicDirManager'));
// eslint-disable-next-line react-refresh/only-export-components
const ApiKeyManager = lazy(() => import('@/components/admin/ApiKeyManager'));
// eslint-disable-next-line react-refresh/only-export-components
const AdminUserManager = lazy(() => import('@/components/admin/AdminUserManager'));
// eslint-disable-next-line react-refresh/only-export-components
const OAuth2ConfigManager = lazy(() => import('@/components/admin/OAuth2ConfigManager'));
// eslint-disable-next-line react-refresh/only-export-components
const MFASettingsManager = lazy(() => import('@/components/admin/MFASettingsManager'));

// eslint-disable-next-line react-refresh/only-export-components
const Loading = () => (
  <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%' }}>
    <Spin />
  </div>
);

const router = createBrowserRouter([
  {
    path: '/share/:code',
    element: (
      <Suspense fallback={<Loading />}>
        <ShareAccessPage />
      </Suspense>
    ),
  },
  {
    path: '/admin/setup',
    element: (
      <Suspense fallback={<Loading />}>
        <AdminSetupPage />
      </Suspense>
    ),
  },
  {
    path: '/admin/login',
    element: (
      <Suspense fallback={<Loading />}>
        <AdminLoginPage />
      </Suspense>
    ),
  },
  {
    path: '/admin',
    element: (
      <Suspense fallback={<Loading />}>
        <AdminPage />
      </Suspense>
    ),
    children: [
      { index: true, element: <Navigate to="/admin/public-dirs" replace /> },
      {
        path: 'public-dirs',
        element: (
          <Suspense fallback={<Loading />}>
            <PublicDirManager />
          </Suspense>
        ),
      },
      {
        path: 'api-keys',
        element: (
          <Suspense fallback={<Loading />}>
            <ApiKeyManager />
          </Suspense>
        ),
      },
      {
        path: 'users',
        element: (
          <Suspense fallback={<Loading />}>
            <AdminUserManager />
          </Suspense>
        ),
      },
      {
        path: 'oauth2',
        element: (
          <Suspense fallback={<Loading />}>
            <OAuth2ConfigManager />
          </Suspense>
        ),
      },
      {
        path: 'mfa',
        element: (
          <Suspense fallback={<Loading />}>
            <MFASettingsManager />
          </Suspense>
        ),
      },
    ],
  },
  {
    path: '/',
    element: (
      <ProtectedRoute>
        <AppLayout />
      </ProtectedRoute>
    ),
    children: [
      { index: true, element: <Navigate to="/explorer" replace /> },
      {
        path: 'explorer',
        element: (
          <Suspense fallback={<Loading />}>
            <ExplorerPage />
          </Suspense>
        ),
      },
      {
        path: 'explorer/:folderId',
        element: (
          <Suspense fallback={<Loading />}>
            <ExplorerPage />
          </Suspense>
        ),
      },
      {
        path: 'public',
        element: (
          <Suspense fallback={<Loading />}>
            <PublicDirectoriesPage />
          </Suspense>
        ),
      },
      {
        path: 'public/:id',
        element: (
          <Suspense fallback={<Loading />}>
            <PublicDirectoriesPage />
          </Suspense>
        ),
      },
      {
        path: 'recycle',
        element: (
          <Suspense fallback={<Loading />}>
            <RecycleBinPage />
          </Suspense>
        ),
      },
      {
        path: 'shares',
        element: (
          <Suspense fallback={<Loading />}>
            <SharesPage />
          </Suspense>
        ),
      },
      {
        path: 'tags',
        element: (
          <Suspense fallback={<Loading />}>
            <TagsPage />
          </Suspense>
        ),
      },
      {
        path: 'permissions',
        element: (
          <Suspense fallback={<Loading />}>
            <PermissionsPage />
          </Suspense>
        ),
      },
      {
        path: 'preview/:fileId',
        element: (
          <Suspense fallback={<Loading />}>
            <PreviewPage />
          </Suspense>
        ),
      },
    ],
  },
]);

export default router;
