export type SidebarCatalogEntry = {
  title: string;
  description: string;
};

export const sidebarCatalog: Readonly<Record<string, SidebarCatalogEntry>> = {
  '/': {
    title: '客户侧边栏',
    description: '查看当前会话客户与可用工作模块。',
  },
  '/auth': {
    title: '企业微信授权回调',
    description: '验证企业微信返回的员工登录状态。',
  },
  '/codeAuth': {
    title: '企业微信扫码授权',
    description: '承接企业微信扫码授权结果。',
  },
  '/contact': {
    title: '客户资料',
    description: '查看当前外部联系人的基础摘要。',
  },
  '/contact/editDetail': {
    title: '编辑客户资料',
    description: '维护当前客户的扩展资料。',
  },
  '/contact/remark': {
    title: '客户备注',
    description: '维护当前员工视角下的客户备注。',
  },
  '/contact/settingTag': {
    title: '设置客户标签',
    description: '查看并维护当前客户的企业标签。',
  },
  '/contactBatchAdd': {
    title: '批量加好友',
    description: '查看批量加好友任务与分配情况。',
  },
  '/contactSop': {
    title: '个人客户 SOP',
    description: '查看当前客户的个人 SOP 任务。',
  },
  '/login': {
    title: '侧边栏登录',
    description: '使用企业微信员工身份进入客户侧边栏。',
  },
  '/medium': {
    title: '素材库',
    description: '查看可发送给客户的企业素材。',
  },
  '/roomSop': {
    title: '客户群 SOP',
    description: '查看当前客户群的 SOP 任务。',
  },
};

// Keep the Phase 2 audit import stable while the functional registry takes ownership.
export const sidebarRoutes = sidebarCatalog;
