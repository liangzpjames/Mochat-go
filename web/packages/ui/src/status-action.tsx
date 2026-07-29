export interface StatusActionProps {
  label: string;
  confirmation: string;
  onConfirm: () => void;
  disabled?: boolean;
}

export function StatusAction({
  label,
  confirmation,
  onConfirm,
  disabled = false,
}: StatusActionProps) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={() => {
        if (window.confirm(confirmation)) {
          onConfirm();
        }
      }}
    >
      {label}
    </button>
  );
}
