import { useQuery } from '@tanstack/react-query';
import { useDashboardAccess } from '../../app/access-context';
import { PageState } from '../../components/page-state/page-state';
import type { ContactApi } from './contact-api';

export function ContactPage({ api }: { api: ContactApi }) {
  const access = useDashboardAccess();
  const query = useQuery({ queryKey: ['scrm-contacts', access.corp.id], queryFn: () => api.listContacts({ corpId: Number(access.corp.id) }) });
  if (query.isPending) return <PageState state="loading" />;
  if (query.isError) return <PageState state="error" onRetry={() => void query.refetch()} />;
  return <section><h1>联系人</h1><table><thead><tr><th>姓名</th><th>电话</th></tr></thead><tbody>{query.data.items.map((item) => <tr key={item.id}><td>{item.name}</td><td>{item.phone ?? '--'}</td></tr>)}</tbody></table></section>;
}
