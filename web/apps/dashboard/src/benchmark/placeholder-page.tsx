import { Link } from 'react-router';

export function PlaceholderPage({
  groupTitle,
  title,
}: {
  groupTitle?: string;
  title: string;
}) {
  return (
    <section className="benchmark-placeholder-page">
      <p className="benchmark-placeholder-breadcrumb">
        {groupTitle === undefined ? '工作台' : `工作台 / ${groupTitle}`}
      </p>
      <h1>{title}</h1>
      <p className="benchmark-placeholder-status">功能建设中</p>
      <p>该功能正在完善，敬请期待。</p>
      <Link to="/">返回工作台</Link>
    </section>
  );
}
