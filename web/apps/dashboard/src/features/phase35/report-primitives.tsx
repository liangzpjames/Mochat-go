import type { ReportResult } from './report-types';

export function ReportPrimitives({ result }: { result?: ReportResult }) {
  const hasSummary = Object.keys(result?.summary ?? {}).length > 0;
  const hasItems = (result?.items ?? []).length > 0;
  const hasLimitations = (result?.limitations ?? []).length > 0;
  if (!result || (!hasSummary && !hasItems && !hasLimitations)) {
    return <p>暂无数据。</p>;
  }

  return (
    <>
      <div className="dashboard-stat-grid">
        {Object.entries(result.summary ?? {}).map(([key, value]) => (
          <article key={key}>
            <span>{key}</span>
            <strong>{value === null ? '--' : value}</strong>
          </article>
        ))}
      </div>
      {result.limitations?.map((limitation) => (
        <p role="status" key={limitation.provider}>{limitation.message}</p>
      ))}
      <table>
        <tbody>
          {(result.items ?? []).map((row, rowIndex) => (
            <tr key={rowIndex}>
              {Object.values(row).map((value, valueIndex) => (
                <td key={valueIndex}>{String(value ?? '--')}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
