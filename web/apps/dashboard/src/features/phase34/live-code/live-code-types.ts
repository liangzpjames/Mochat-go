export type LiveCodeRecord = Record<string, unknown>;

export type LiveCodeKind = 'channel' | 'group';

export type LiveCodeState = 'active' | 'paused' | 'expired' | 'draft' | 'unknown';

export type LiveCodeQuery = {
  name?: string;
  creator?: string;
  employee?: string;
  state?: string;
  groupId?: number;
  page: number;
  perPage: number;
};

export type LiveCodeSummary = {
  addedAttempts: number;
  lostAttempts: number;
  addedCustomers: number;
  retainedCustomers: number;
  codeCount: number;
  available: boolean;
  asOf: string;
  timezone: string;
  definition: string;
};

export type LiveCodeStatisticsPage = {
  summary: LiveCodeSummary;
  rows: LiveCodeRecord[];
  total: number;
  totalPage: number;
  perPage: number;
};
