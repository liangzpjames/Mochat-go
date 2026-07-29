import { describe, expect, it, vi } from 'vitest';

import { createPasswordApi } from './password-api';

describe('password api', () => {
  it('uses the audited password update contract', async () => {
    const request = vi.fn(() => Promise.resolve([]));
    const api = createPasswordApi({ request });
    const input = {
      oldPassword: 'old123',
      newPassword: 'new456',
      againNewPassword: 'new456',
    };

    await api.update(input);

    expect(request).toHaveBeenCalledWith('/user/passwordUpdate', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
    });
  });
});
