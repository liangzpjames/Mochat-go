export type ErrorStateProps = {
  title?: string;
  message: string;
};

export function ErrorState({ title = '页面暂时不可用', message }: ErrorStateProps) {
  return (
    <section role="alert">
      <h2>{title}</h2>
      <p>{message}</p>
    </section>
  );
}
