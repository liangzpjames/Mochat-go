import type { AccessCatalogItem, EffectivePermission } from "./access-api";

type Client = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};
const json = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

export type AccessRoleSummary = {
  id: number;
  name: string;
  status: number;
  version: number;
};
export type AccessRole = AccessRoleSummary & {
  remark: string;
  isSystem: boolean;
  memberCount: number;
  permissions: { code: string; scope: "self" | "department" | "tenant" }[];
};
export type AccessUserSummary = {
  id: number;
  name: string;
  phone: string;
  status: number;
  isSuperAdmin: boolean;
  version: number;
};
export type AccessUser = AccessUserSummary & {
  roles: AccessRoleSummary[];
  directPermissions: {
    code: string;
    scope: "self" | "department" | "tenant";
  }[];
  inheritedPermissions: EffectivePermission[];
  effectivePermissions: EffectivePermission[];
};
export type AccessEmployeeAccount = {
  userId: number;
  loginIdentifier: string;
  status: number;
  mustRotatePassword: boolean;
  authVersion: number;
};
export type AccessEmployee = {
  id: number;
  wxUserId: string;
  name: string;
  mobile: string;
  status: number;
  account: AccessEmployeeAccount | null;
};
export type AccessEmployeeMutationResult = {
  employee: AccessEmployee;
  temporaryPassword?: string;
};
export type Page<T> = {
  list: T[];
  page: { page: number; perPage: number; total: number; totalPage: number };
};
export type AccessAudit = {
  id: number;
  actorUserId: number | null;
  actorName: string;
  action: string;
  targetType: string;
  targetId: string;
  before: unknown;
  after: unknown;
  expectedVersion: number | null;
  resultVersion: number | null;
  requestId: string;
  time: string;
};
export type DashboardAccessAdminApi = ReturnType<
  typeof createDashboardAccessAdminApi
>;

export function createDashboardAccessAdminApi(client: Client) {
  return {
    employees: (input: {
      page: number;
      perPage: number;
    }): Promise<Page<AccessEmployee>> =>
      client.request(
        `/access/employees?page=${input.page}&perPage=${input.perPage}`,
      ) as Promise<Page<AccessEmployee>>,
    provisionEmployeeAccount: (
      id: number,
      input: {
        loginIdentifier: string;
        roleIds: number[];
        directPermissions: { code: string; scope: string }[];
      },
    ) =>
      client.request(
        `/access/employees/${id}/account`,
        json("POST", input),
      ) as Promise<AccessEmployeeMutationResult>,
    updateEmployeeAccountStatus: (id: number, status: 1 | 2) =>
      client.request(
        `/access/employees/${id}/account/status`,
        json("PUT", { status }),
      ) as Promise<AccessEmployeeMutationResult>,
    resetEmployeePassword: (id: number) =>
      client.request(
        `/access/employees/${id}/account/reset-password`,
        json("POST", {}),
      ) as Promise<AccessEmployeeMutationResult>,
    users: (input: {
      page: number;
      perPage: number;
    }): Promise<Page<AccessUserSummary>> =>
      client.request(
        `/access/users?page=${input.page}&perPage=${input.perPage}`,
      ) as Promise<Page<AccessUserSummary>>,
    user: (id: number): Promise<AccessUser> =>
      client.request(`/access/users/${id}`) as Promise<AccessUser>,
    replaceUser: (
      id: number,
      input: {
        roleIds: number[];
        directPermissions: { code: string; scope: string }[];
        expectedVersion: number;
      },
    ) =>
      client.request(
        `/access/users/${id}`,
        json("PUT", input),
      ) as Promise<AccessUser>,
    roles: (input: {
      page: number;
      perPage: number;
    }): Promise<Page<AccessRole>> =>
      client.request(
        `/access/roles?page=${input.page}&perPage=${input.perPage}`,
      ) as Promise<Page<AccessRole>>,
    createRole: (input: {
      name: string;
      remark: string;
      status: number;
      permissions: { code: string; scope: string }[];
    }) =>
      client.request(
        "/access/roles",
        json("POST", input),
      ) as Promise<AccessRole>,
    updateRole: (
      id: number,
      input: {
        name: string;
        remark: string;
        permissions: { code: string; scope: string }[];
        expectedVersion: number;
      },
    ) =>
      client.request(
        `/access/roles/${id}`,
        json("PUT", input),
      ) as Promise<AccessRole>,
    updateRoleStatus: (
      id: number,
      input: { status: number; expectedVersion: number },
    ) =>
      client.request(
        `/access/roles/${id}/status`,
        json("PUT", input),
      ) as Promise<AccessRole>,
    deleteRole: (id: number, expectedVersion: number) =>
      client.request(
        `/access/roles/${id}`,
        json("DELETE", { expectedVersion }),
      ),
    catalog: (): Promise<AccessCatalogItem[]> =>
      client.request("/access/catalog") as Promise<AccessCatalogItem[]>,
    audits: (input: {
      page: number;
      perPage: number;
    }): Promise<Page<AccessAudit>> =>
      client.request(
        `/access/audits?page=${input.page}&perPage=${input.perPage}`,
      ) as Promise<Page<AccessAudit>>,
  };
}
