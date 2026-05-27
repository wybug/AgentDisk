import { defineConfig } from 'vitepress';

export default defineConfig({
  lang: 'zh-CN',
  title: 'AgentDisk 文档',
  description: '专为多智能体设计的企业级云盘中间件',
  vite: {
    server: {
      port: 9102,
    },
  },
  themeConfig: {
    nav: [
      { text: '使用指南', link: '/guide/getting-started' },
      { text: 'API 参考', link: '/api/overview' },
      { text: '集成指南', link: '/integration/overview' },
      { text: '架构文档', link: '/architecture/overview' },
      {
        text: 'OpenAPI',
        link: 'https://petstore.swagger.io/?url=https://raw.githubusercontent.com/anthropics/agent-disk/main/docs/openapi.yaml',
      },
    ],
    sidebar: {
      '/guide/': [
        {
          text: '开始使用',
          items: [
            { text: '快速开始', link: '/guide/getting-started' },
            { text: '安装部署', link: '/guide/installation' },
            { text: '配置说明', link: '/guide/configuration' },
          ],
        },
        {
          text: '功能指南',
          items: [
            { text: '文件管理器', link: '/guide/file-explorer' },
            { text: '公共目录', link: '/guide/public-directories' },
            { text: '回收站', link: '/guide/recycle-bin' },
            { text: '外链分享', link: '/guide/share-links' },
            { text: '标签搜索', link: '/guide/tags' },
            { text: '版本回溯', link: '/guide/file-versions' },
            { text: '权限管理', link: '/guide/permissions' },
            { text: '管理后台', link: '/guide/admin-panel' },
          ],
        },
      ],
      '/api/': [
        {
          text: 'API 参考',
          items: [
            { text: 'API 概览', link: '/api/overview' },
            { text: '空间接口', link: '/api/space' },
            { text: '目录接口', link: '/api/folders' },
            { text: '文件接口', link: '/api/files' },
            { text: '分享接口', link: '/api/shares' },
            { text: '权限接口', link: '/api/permissions' },
            { text: '标签接口', link: '/api/tags' },
            { text: '版本接口', link: '/api/versions' },
            { text: '回收站接口', link: '/api/recycle' },
            { text: '预览接口', link: '/api/preview' },
            { text: '管理接口', link: '/api/admin' },
          ],
        },
      ],
      '/integration/': [
        {
          text: '集成指南',
          items: [
            { text: '集成概览', link: '/integration/overview' },
            { text: '认证集成', link: '/integration/auth' },
            { text: 'Agent 集成', link: '/integration/agent' },
            { text: 'Python SDK', link: '/integration/sdk-python' },
            { text: 'API Key 认证', link: '/integration/api-keys' },
          ],
        },
      ],
      '/architecture/': [
        {
          text: '架构文档',
          items: [
            { text: '架构概览', link: '/architecture/overview' },
            { text: '公共目录架构', link: '/architecture/public-directory' },
            { text: '安全设计', link: '/architecture/security' },
          ],
        },
      ],
    },
    search: {
      provider: 'local',
    },
    socialLinks: [{ icon: 'github', link: 'https://github.com/anthropics/agent-disk' }],
    footer: {
      message: '基于 Apache 2.0 许可发布',
    },
  },
});
