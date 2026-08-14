import { MobileCard, MobileShell, MobileState } from '@mochat/mobile-foundation';
import type { ReactNode } from 'react';
import { createBrowserRouter } from 'react-router';

import { type WorkFissionRequest } from '../features/work-fission/work-fission-api';
import { WorkFissionPage } from '../features/work-fission/work-fission-page';
import {
  operationRouteRegistry,
  type OperationRouteRegistration,
} from '../routes/registry';

export type OperationRuntime = {
  basename: string;
  request: WorkFissionRequest;
};

function PendingActivityPage({ route }: { route: OperationRouteRegistration }) {
  return (
    <MobileShell
      appName="MoChat 营销活动"
      title={route.title}
      eyebrow="营销活动"
      subtitle={route.description}
      hero={(
        <div className="operation-hero operation-hero--pending" aria-hidden="true">
          <div className="operation-hero__copy">
            <strong>{route.title}</strong>
            <span>已保留入口，等待真实业务迁移</span>
          </div>
          <span className="operation-hero__orb" />
        </div>
      )}
    >
      <section className="operation-pending" aria-label={`${route.title}状态`}>
        <MobileCard tone="surface" padding="comfortable">
          <MobileState
            kind="empty"
            title={`${route.title}模块待迁移`}
            description="该历史入口已由新路由承接，业务功能将在后续迁移。"
          />
        </MobileCard>
      </section>
    </MobileShell>
  );
}

function routeElement(route: OperationRouteRegistration, runtime: OperationRuntime): ReactNode {
  if (route.requiresActivitySession) {
    if (route.moduleKey !== 'work-fission-activity' || route.activityKind !== 'workFission') {
      throw new Error(`Operation route ${route.path} has unsupported activity-session metadata.`);
    }
    return <WorkFissionPage activityKind={route.activityKind} request={runtime.request} />;
  }
  return <PendingActivityPage route={route} />;
}

function OperationNotFoundPage() {
  return (
    <MobileShell appName="MoChat 营销活动" title="未找到活动页面">
      <MobileState
        kind="not-found"
        title="页面不存在"
        description="请检查活动访问地址。"
      />
    </MobileShell>
  );
}

export function createOperationRouter(runtime: OperationRuntime) {
  return createBrowserRouter([
    ...operationRouteRegistry.map((route) => ({
      path: route.path,
      element: routeElement(route, runtime),
    })),
    { path: '*', element: <OperationNotFoundPage /> },
  ], { basename: runtime.basename });
}
