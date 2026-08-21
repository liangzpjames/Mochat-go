import { useEffect, useState } from 'react';

import type { StaffDepartment } from './conversation-global-api';

type Props = {
  rows: readonly StaffDepartment[];
  selectedDepartmentId: number | null;
  onSelect: (id: number | null) => void;
};

function selectedName(rows: readonly StaffDepartment[], selectedId: number | null): string {
  if (selectedId === null) return '全部部门';
  for (const row of rows) {
    if (row.id === selectedId) return row.name;
    const childName = selectedName(row.children, selectedId);
    if (childName !== '全部部门') return childName;
  }
  return '全部部门';
}

function DepartmentOptions(props: {
  rows: readonly StaffDepartment[];
  selectedDepartmentId: number | null;
  onSelect: (id: number) => void;
}) {
  return <>
    {props.rows.map((row) => <div className="employee-conversation-department-node" key={row.id}>
      <button aria-selected={props.selectedDepartmentId === row.id} role="option" onClick={() => props.onSelect(row.id)} type="button">
        {row.name}
      </button>
      {row.children.length > 0 && <div><DepartmentOptions onSelect={props.onSelect} rows={row.children} selectedDepartmentId={props.selectedDepartmentId} /></div>}
    </div>)}
  </>;
}

export function EmployeeConversationDepartmentPicker(props: Props) {
  const [open, setOpen] = useState(false);
  const label = selectedName(props.rows, props.selectedDepartmentId);

  useEffect(() => {
    if (!open) return undefined;
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === 'Escape') setOpen(false); };
    window.addEventListener('keydown', closeOnEscape);
    return () => window.removeEventListener('keydown', closeOnEscape);
  }, [open]);

  function select(id: number | null) {
    setOpen(false);
    props.onSelect(id);
  }

  return <div className="employee-conversation-department-picker">
    <button aria-expanded={open} aria-haspopup="listbox" aria-label={`部门：${label}`} className="employee-conversation-department-trigger" onClick={() => setOpen((value) => !value)} type="button">
      <span>部门：{label}</span><span aria-hidden="true">⌄</span>
    </button>
    {open && <div aria-label="部门选择" className="employee-conversation-department-menu" role="listbox">
      <button aria-selected={props.selectedDepartmentId === null} role="option" onClick={() => select(null)} type="button">全部部门</button>
      <DepartmentOptions onSelect={select} rows={props.rows} selectedDepartmentId={props.selectedDepartmentId} />
    </div>}
  </div>;
}
