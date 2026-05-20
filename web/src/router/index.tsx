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
