import { Button } from 'antd';

export function RowAction({
  action,
  target,
  onClick,
  danger = false,
  disabled = false,
  loading = false,
}: {
  action: string;
  target: string;
  onClick: () => void;
  danger?: boolean;
  disabled?: boolean;
  loading?: boolean;
}) {
  return (
    <Button
      aria-label={`${action} ${target}`}
      danger={danger}
      disabled={disabled}
      loading={loading}
      type="link"
      onClick={onClick}
    >
      {action}
    </Button>
  );
}
