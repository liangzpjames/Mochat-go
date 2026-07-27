export type LoadingStateProps = {
  label?: string;
};

export function LoadingState({ label = '正在加载…' }: LoadingStateProps) {
  return (
    <div aria-live="polite" role="status">
      {label}
    </div>
  );
}
