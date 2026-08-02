import { ApiError } from '@mochat/api-client';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AssignmentEditor } from './assignment-editor';

describe('AssignmentEditor', () => {
  it('submits owner, collaborators, and optimistic version', async () => {
    const updateAssignment = vi.fn().mockResolvedValue({});
    render(<AssignmentEditor assignment={{ id: 'a1', contactId: 'c1', ownerId: null, collaboratorIds: [], status: 'public_pool', version: 4 }} corpId={7} api={{ updateAssignment }} />);
    fireEvent.change(screen.getByLabelText('负责人'), { target: { value: '12' } });
    fireEvent.change(screen.getByLabelText('协作人'), { target: { value: '13, 14' } });
    fireEvent.click(screen.getByRole('button', { name: '保存分配' }));
    await waitFor(() => expect(updateAssignment).toHaveBeenCalledWith({ corpId: 7, contactId: 'c1', ownerId: 12, collaboratorIds: [13, 14], version: 4, idempotencyKey: 'assignment-c1-4' }));
  });

  it.each([
    [403, 'forbidden'],
    [404, 'not-found'],
    [409, 'conflict'],
    [500, 'error'],
  ])('renders status %i through PageState and offers retry', async (status, state) => {
    const updateAssignment = vi.fn().mockRejectedValue(new ApiError('validation', 'failed', { status }));
    const { container } = render(<AssignmentEditor assignment={{ id: 'a1', contactId: 'c1', ownerId: null, collaboratorIds: [], status: 'public_pool', version: 4 }} corpId={7} api={{ updateAssignment }} />);
    fireEvent.click(screen.getByRole('button', { name: '保存分配' }));
    await waitFor(() => expect(container.querySelector(`.page-state-${state}`)).not.toBeNull());
    fireEvent.click(container.querySelector('.page-state-retry')!);
    await waitFor(() => expect(updateAssignment).toHaveBeenCalledTimes(2));
  });

  it('renders pending save through PageState loading', async () => {
    const updateAssignment = vi.fn().mockReturnValue(new Promise(() => undefined));
    const { container } = render(<AssignmentEditor assignment={{ id: 'a1', contactId: 'c1', ownerId: null, collaboratorIds: [], status: 'public_pool', version: 4 }} corpId={7} api={{ updateAssignment }} />);
    fireEvent.click(screen.getByRole('button', { name: '保存分配' }));
    await waitFor(() => expect(container.querySelector('.page-state-loading')).not.toBeNull());
  });
});

afterEach(cleanup);
