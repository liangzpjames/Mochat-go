import { render, screen } from '@testing-library/react';
import type { Session } from '@mochat/auth';
import { describe, expect, it } from 'vitest';

import {
  DashboardAccessProvider,
  useDashboardAccess,
} from './access-context';
import type { AccessContext } from './access-loader';

const session: Session = {
  token: 'token',
  userId: '7',
  corpId: '3',
  expiresAt: null,
};

function AccessProbe() {
  const access = useDashboardAccess();
  return (
    <div>
      {access.corp.name}:{String(access.allowedActions.has('/contact@delete'))}
    </div>
  );
}

describe('DashboardAccessProvider', () => {
  it('makes corp, route, and action permissions available to the shell', () => {
    const value: AccessContext = {
      session,
      corp: { id: '3', name: '迁移企业', authorized: true },
      menu: [],
      allowedRoutes: new Set(['/contact/index']),
      allowedActions: new Set(['/contact@delete']),
    };

    render(
      <DashboardAccessProvider value={value}>
        <AccessProbe />
      </DashboardAccessProvider>,
    );

    expect(screen.getByText('迁移企业:true')).toBeTruthy();
  });
});
