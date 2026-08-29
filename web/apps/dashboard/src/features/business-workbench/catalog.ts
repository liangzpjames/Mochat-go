import migrationRoutes from '../../migration-routes.json';

export type BusinessPageMode = 'list' | 'form' | 'detail' | 'statistics';
export type BusinessField = {
  key: string;
  label: string;
  kind: 'text' | 'select' | 'textarea';
  options?: string[];
};
export type BusinessAction = 'search' | 'reset' | 'create' | 'save' | 'back' | 'sync' | 'refresh';
export type BusinessRouteConfig = {
  path: string;
  title: string;
  description: string;
  mode: BusinessPageMode;
  readEndpoint: string;
  writeEndpoint?: string;
  fields: BusinessField[];
  actions: BusinessAction[];
};

export const specializedDashboardRoutes = new Set([
  '/company-setting/website',
  '/contactField/index',
  '/department/index',
  '/menu/index',
  '/passwordUpdate/index',
  '/role/index',
  '/role/permissionShow',
  '/user/index',
  '/workContactTag/index',
  '/workEmployee/index',
]);

const featureNames: Record<string, string> = {
  autoTag: '自动标签',
  channelCode: '渠道活码',
  chatTool: '聊天工具栏',
  contactMessageBatchSend: '客户群发',
  contactTransfer: '离职继承',
  corpData: '企业数据',
  greeting: '好友欢迎语',
  lossContact: '流失客户',
  mediumGroup: '素材库',
  officialAccount: '公众号',
  roomMessageBatchSend: '客户群群发',
  roomTagPull: '标签建群',
  roomWelcome: '入群欢迎语',
  statistics: '数据统计',
  workContact: '客户管理',
  workFission: '任务宝',
  workRoom: '客户群',
  workRoomAutoPull: '自动拉群',
};

const pageNames: Record<string, string> = {
  contact: '客户统计',
  contactDetail: '客户详情',
  contactFieldPivot: '客户画像',
  create: '新建',
  customer: '客户配置',
  dataShow: '活动数据',
  dayPartCreate: '新建时段标签',
  dayPartIndex: '时段标签',
  dayPartShow: '时段标签详情',
  detail: '详情',
  edit: '编辑',
  employee: '员工统计',
  enhance: '效率工具',
  index: '管理',
  invite: '邀请记录',
  joinRoomCreate: '新建入群标签',
  joinRoomIndex: '入群标签',
  joinRoomShow: '入群标签详情',
  keywordCreate: '新建关键词标签',
  keywordIndex: '关键词标签',
  keywordShow: '关键词标签详情',
  resignAllotRecord: '离职分配记录',
  resignIndex: '离职员工分配',
  show: '详情',
  statistics: '统计',
  store: '新建',
  taskpage: '活动任务',
  workAllotRecord: '在职分配记录',
  workIndex: '在职员工分配',
};

function modeFor(page: string): BusinessPageMode {
  if (['create', 'store', 'edit'].includes(page) || page.endsWith('Create')) {
    return 'form';
  }
  if (page.includes('statistics') || ['contact', 'employee', 'dataShow'].includes(page)) {
    return 'statistics';
  }
  if (['show', 'detail', 'invite', 'contactDetail'].includes(page) || page.endsWith('Show')) {
    return 'detail';
  }
  return 'list';
}

function readEndpoint(feature: string, page: string, mode: BusinessPageMode): string {
  if (mode === 'form') {
    return `/${feature}/show`;
  }
  return `/${feature}/${page}`;
}

function fieldsFor(mode: BusinessPageMode): BusinessField[] {
  if (mode === 'form') {
    return [
      { key: 'name', label: '名称', kind: 'text' },
      { key: 'content', label: '内容', kind: 'textarea' },
      { key: 'status', label: '状态', kind: 'select', options: ['启用', '停用'] },
    ];
  }
  return [
    { key: 'keyword', label: '关键词', kind: 'text' },
    { key: 'status', label: '状态', kind: 'select', options: ['全部', '启用', '停用'] },
  ];
}

function actionsFor(mode: BusinessPageMode): BusinessAction[] {
  if (mode === 'form') return ['save', 'back'];
  if (mode === 'detail') return ['refresh', 'back'];
  if (mode === 'statistics') return ['search', 'reset', 'refresh'];
  return ['search', 'reset', 'create', 'sync'];
}

export const businessRouteCatalog: Readonly<Record<string, BusinessRouteConfig>> =
  Object.fromEntries(
    migrationRoutes
      .map((route) => route.path)
      .filter((path) => !specializedDashboardRoutes.has(path))
      .map((path) => {
        const [feature = '', page = 'index'] = path.split('/').filter(Boolean);
        const mode = modeFor(page);
        const title = `${featureNames[feature] ?? feature}${pageNames[page] ?? page}`;
        return [path, {
          path,
          title,
          description: `${title}，保留旧版业务入口、筛选条件和基础操作，可在此基础上继续扩展业务规则。`,
          mode,
          readEndpoint: readEndpoint(feature, page, mode),
          ...(mode === 'form' ? { writeEndpoint: `/${feature}/${page === 'edit' ? 'update' : 'store'}` } : {}),
          fields: fieldsFor(mode),
          actions: actionsFor(mode),
        } satisfies BusinessRouteConfig];
      }),
  );
