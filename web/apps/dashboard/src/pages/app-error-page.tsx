export type AppErrorPageProps = {
  message: string;
  retry: () => void;
};

export function AppErrorPage({ message, retry }: AppErrorPageProps) {
  return (
    <main role="alert">
      <h1>加载失败</h1>
      <p>{message}</p>
      <button onClick={retry} type="button">重试</button>
    </main>
  );
}
