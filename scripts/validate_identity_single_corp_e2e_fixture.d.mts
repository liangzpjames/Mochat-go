export const REQUIRED_ACCOUNT_NAMES: readonly string[];

export function validateFixture(fixture: unknown): {
  accounts: number;
  expectedTenantId: number;
  expectedCorpId: number;
  expectedWxCorpId: string;
};

export function resolveFixtureCredentials(
  reference: unknown,
  environment?: Record<string, string | undefined>,
): { login: string; password: string };

export function loadFixture(filePath: string): unknown;
