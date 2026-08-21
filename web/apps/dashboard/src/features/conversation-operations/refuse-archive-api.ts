export type RefuseArchiveSubjectType = 'customer' | 'room';

export type RefuseArchiveRecord = {
  id: number;
  subjectType: string;
  subjectId: string;
  subjectName: string;
  employeeId: number;
  employeeName: string;
  authorizationStatus: string;
  source: string;
  refusedAt: string;
  authorizedAt: string;
  lastFollowUpAt: string;
  followUpStatus: string;
  followUpNote: string;
  updatedAt: string;
};

export type RefuseArchiveFilter = {
  subjectType: RefuseArchiveSubjectType;
  subject: string;
  employeeId: number | '';
  refusedFrom: string;
  refusedTo: string;
  authorizationStatus: string;
  followUpStatus: string;
  page: number;
  perPage: number;
};

export type RefuseArchivePage = {
  items: readonly RefuseArchiveRecord[];
  total: number;
  page: number;
  perPage: number;
};

type Client = { request<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T> };

export function createRefuseArchiveApi(client: Client) {
  return {
    list(filter: RefuseArchiveFilter): Promise<RefuseArchivePage> {
      const query = new URLSearchParams({ subjectType: filter.subjectType });
      if (filter.subject.trim()) query.set('subject', filter.subject.trim());
      if (filter.employeeId !== '') query.set('employeeId', String(filter.employeeId));
      if (filter.refusedFrom) query.set('refusedFrom', filter.refusedFrom);
      if (filter.refusedTo) query.set('refusedTo', filter.refusedTo);
      if (filter.authorizationStatus) query.set('authorizationStatus', filter.authorizationStatus);
      if (filter.followUpStatus) query.set('followUpStatus', filter.followUpStatus);
      query.set('page', String(filter.page));
      query.set('perPage', String(filter.perPage));
      return client.request<RefuseArchivePage>(`/refuse-archive/records?${query.toString()}`);
    },
    followUp(input: { id: number; status: string; note: string }): Promise<{ updated: boolean }> {
      return client.request<{ updated: boolean }>('/refuse-archive/follow-up', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(input),
      });
    },
  };
}

export type RefuseArchiveApi = ReturnType<typeof createRefuseArchiveApi>;
