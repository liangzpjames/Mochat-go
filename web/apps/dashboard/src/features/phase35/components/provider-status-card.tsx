export function ProviderStatusCard({ available }: { available: boolean }) {
  return (
    <section aria-label="会话数据状态" className="dashboard-card">
      <h2>会话归档数据</h2>
      <strong>{available ? '已同步' : '待同步'}</strong>
      {!available && <p>完成会话存档配置并同步数据后，这里会展示员工、消息与响应指标。</p>}
    </section>
  );
}
