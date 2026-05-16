import { createBrowserRouter, Navigate } from 'react-router-dom';
import { lazy, Suspense } from 'react';
import { Spin } from 'antd';
import ProtectedRoute from './ProtectedRoute';
import AppLayout from '@/components/layout/AppLayout';

const ExplorerPage = lazy(() => import('@/pages/ExplorerPage'));
const RecycleBinPage = lazy(() => import('@/pages/RecycleBinPage'));
const SharesPage = lazy(() => import('@/pages/SharesPage'));
const TagsPage = lazy(() => import('@/pages/TagsPage'));
const PermissionsPage = lazy(() => import('@/pages/PermissionsPage'));
const ShareAccessPage = lazy(() => import('@/pages/ShareAccessPage'));
const PreviewPage = lazy(() => import('@/pages/PreviewPage'));

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
