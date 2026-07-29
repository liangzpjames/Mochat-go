export type SidebarRouteConfig = {
  title: string;
  description: string;
  sections: string[];
  actions: string[];
};

export const sidebarRoutes: Readonly<Record<string, SidebarRouteConfig>> = {
  '/': {
    title: '客户侧边栏',
    description: '快速查看当前客户资料和待办事项。',
    sections: ['客户摘要', '最近互动', '快捷操作'],
    actions: ['查看客户'],
  },
  '/auth': {
    title: '企业微信授权',
    description: '使用 fake 企业微信完成侧边栏授权回调。',
    sections: ['授权应用', '登录企业', '授权状态'],
    actions: ['确认授权'],
  },
  '/codeAuth': {
    title: '扫码授权',
    description: '扫描测试二维码并建立 fake 外部平台会话。',
    sections: ['授权二维码', '扫码状态'],
    actions: ['刷新二维码'],
  },
  '/contact': {
    title: '客户资料',
    description: '查看客户身份、标签、自定义字段及互动轨迹。',
    sections: ['基本信息', '客户标签', '自定义字段', '互动轨迹'],
    actions: ['修改备注', '设置标签', '编辑资料'],
  },
  '/contact/editDetail': {
    title: '编辑客户资料',
    description: '维护客户画像和自定义字段。',
    sections: ['基础资料', '客户画像'],
    actions: ['保存资料', '返回'],
  },
  '/contact/remark': {
    title: '客户备注',
    description: '修改当前员工视角下的客户备注。',
    sections: ['当前备注', '备注历史'],
    actions: ['保存备注', '返回'],
  },
  '/contact/settingTag': {
    title: '设置客户标签',
    description: '选择企业标签并同步到 fake 企业微信。',
    sections: ['已选标签', '企业标签'],
    actions: ['保存标签', '返回'],
  },
  '/contactBatchAdd': {
    title: '批量加好友',
    description: '查看批量加好友任务的客户和执行状态。',
    sections: ['任务信息', '客户列表', '执行进度'],
    actions: ['刷新进度'],
  },
  '/contactSop': {
    title: '个人 SOP',
    description: '查看针对当前客户的个人 SOP 推送计划。',
    sections: ['待执行任务', '已完成任务', '素材内容'],
    actions: ['标记完成', '查看素材'],
  },
  '/login': {
    title: '侧边栏登录',
    description: '使用测试账号进入客户侧边栏。',
    sections: ['账号登录'],
    actions: ['登录'],
  },
  '/medium': {
    title: '素材库',
    description: '浏览文本、图片、视频、文件和链接素材。',
    sections: ['素材分组', '最近使用', '素材列表'],
    actions: ['发送素材', '搜索素材'],
  },
  '/roomSop': {
    title: '群 SOP',
    description: '查看群聊 SOP、群日历和群质检任务。',
    sections: ['群 SOP', '群日历', '群质检', '推送规则'],
    actions: ['标记完成', '查看详情'],
  },
};
