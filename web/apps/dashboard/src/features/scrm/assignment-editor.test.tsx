import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { AssignmentEditor } from './assignment-editor';

describe('AssignmentEditor', () => {
  it('submits owner, collaborators, and optimistic version', async () => {
    const updateAssignment = vi.fn().mockResolvedValue({});
    render(<AssignmentEditor assignment={{ id: 'a1', contactId: 'c1', ownerId: null, collaboratorIds: [], status: 'public_pool', version: 4 }} corpId={7} api={{ updateAssignment, listPublicPool: vi.fn(), releaseToPublicPool: vi.fn(), claimFromPublicPool: vi.fn() }} />);
    fireEvent.change(screen.getByLabelText('负责人'), { target: { value: '12' } });
    fireEvent.change(screen.getByLabelText('协作人'), { target: { value: '13, 14' } });
    fireEvent.click(screen.getByRole('button', { name: '保存分配' }));
    await waitFor(() => expect(updateAssignment).toHaveBeenCalledWith({ corpId: 7, contactId: 'c1', ownerId: 12, collaboratorIds: [13, 14], version: 4, idempotencyKey: 'assignment-c1-4' }));
  });
});
