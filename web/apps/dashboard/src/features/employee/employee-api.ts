export type EmployeeListInput = {
  name: string;
  status: number | null;
  contactAuth: number | null;
  page: number;
  perPage: number;
};

export type EmployeeListItem = {
  id: number;
  name: string;
  thumbAvatar: string;
  statusName: string;
  contactAuthName: string;
  gender: string;
  applyNums: number;
  addNums: number;
  messageNums: number;
  sendMessageNums: number;
  replyMessageRatio: string;
  averageReply: number;
  invalidContact: number;
};

export type EmployeeListResult = {
  list: EmployeeListItem[];
  page: { page: number; perPage: number; total: number; totalPage: number };
};

export type EmployeeOption = { id: number; name: string };
export type EmployeeConditions = {
  status: EmployeeOption[];
  contactAuth: EmployeeOption[];
  syncTime: string;
};

type ApiClient = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};

export function createEmployeeApi(client: ApiClient) {
  return {
    list(input: EmployeeListInput): Promise<EmployeeListResult> {
      const query = new URLSearchParams({ name: input.name });
      if (input.status !== null) query.set('status', String(input.status));
      if (input.contactAuth !== null) query.set('contactAuth', String(input.contactAuth));
      query.set('page', String(input.page));
      query.set('perPage', String(input.perPage));
      return client.request(`/workEmployee/index?${query.toString()}`) as Promise<EmployeeListResult>;
    },
    conditions(): Promise<EmployeeConditions> {
      return client.request('/workEmployee/searchCondition') as Promise<EmployeeConditions>;
    },
    async sync(): Promise<void> {
      await client.request('/company/employee-sync', { method: 'POST' });
    },
  };
}
