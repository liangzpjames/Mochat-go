import { useState } from 'react';
import type { Assignment, ScrmApi } from './scrm-api';

export function AssignmentEditor({ assignment, corpId, api, canEdit = true, onSaved }: { assignment: Assignment; corpId: number; api: Pick<ScrmApi, 'updateAssignment'>; canEdit?: boolean; onSaved?: (assignment: Assignment) => void }) {
  const [ownerId, setOwnerId] = useState(assignment.ownerId === null ? '' : String(assignment.ownerId));
  const [collaborators, setCollaborators] = useState(assignment.collaboratorIds.join(','));
  const [conflict, setConflict] = useState(false);
  const save = async () => {
    try {
      const next = await api.updateAssignment({ corpId, contactId: assignment.contactId, ownerId: ownerId.trim() ? Number(ownerId) : null, collaboratorIds: collaborators.split(',').map((value) => Number(value.trim())).filter((value) => value > 0), version: assignment.version, idempotencyKey: `assignment-${assignment.contactId}-${assignment.version}` });
      setConflict(false); onSaved?.(next);
    } catch { setConflict(true); }
  };
  return <fieldset disabled={!canEdit}><legend>分配客户</legend><label>负责人<input aria-label="负责人" value={ownerId} onChange={(event) => setOwnerId(event.target.value)} /></label><label>协作人<input aria-label="协作人" value={collaborators} onChange={(event) => setCollaborators(event.target.value)} /></label><button type="button" onClick={() => void save()}>保存分配</button>{conflict && <p role="alert">数据已更新，请刷新当前客户后重试。</p>}</fieldset>;
}
