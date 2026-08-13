export type DashboardResponseStatus = {
  method: string;
  url: string;
  status: number;
  machineCode?: string;
};

export type DashboardEvidencePage = {
  route: string;
  action: string;
  titleVisible: boolean;
  responseStatuses: DashboardResponseStatus[];
  unexpectedResponses: DashboardResponseStatus[];
  consoleErrors: string[];
  pageErrors: string[];
  screenshot: string;
  returnedToIndex: boolean;
  userNameVisible: boolean;
  corpNameVisible: boolean;
};

export type DashboardEvidenceInput = {
  manifestRoutes: string[];
  actionRegistry: Array<{ route: string; action: string }>;
  evidence: DashboardEvidencePage[];
  expectedResponses?: Array<DashboardResponseStatus & { route: string }> | readonly (DashboardResponseStatus & { route: string })[];
};

export function validateDashboardAllPagesEvidence(input: DashboardEvidenceInput): {
  pageCount: number;
  expectedResponseCount: number;
};

export function validateDashboardAllPagesEvidenceFiles(
  input: DashboardEvidenceInput,
  options: { evidenceRoot: string; stat?: (path: string) => Promise<{ isFile(): boolean; size: number }> },
): Promise<{ pageCount: number; expectedResponseCount: number }>;
