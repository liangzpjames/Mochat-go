import * as React from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { DashboardDialog } from "../../components/dashboard-dialog";
import { ConfirmAction } from "../../components/confirm-action";
import { DashboardPagination } from "../../components/dashboard-pagination";
import { Phase35PageShell } from "../phase35/components/phase35-page-shell";
import type { AccessCatalogItem } from "../access/access-api";
import type {
  AccessEmployee,
  AccessEmployeeMutationResult,
  AccessRoleSummary,
  AccessUser,
} from "../access/access-admin-api";
import { AccessPermissionSelector } from "./access-permission-selector";

type Api = {
  employees: (input: { page: number; perPage: number }) => Promise<{
    list: AccessEmployee[];
    page: { total: number; totalPage: number };
  }>;
  provisionEmployeeAccount: (
    id: number,
    input: {
      loginIdentifier: string;
      roleIds: number[];
      directPermissions: { code: string; scope: string }[];
    },
  ) => Promise<AccessEmployeeMutationResult>;
  updateEmployeeAccountStatus: (
    id: number,
    status: 1 | 2,
  ) => Promise<AccessEmployeeMutationResult>;
  resetEmployeePassword: (
    id: number,
  ) => Promise<AccessEmployeeMutationResult>;
  user: (id: number) => Promise<AccessUser>;
  replaceUser: (
    id: number,
    input: {
      roleIds: number[];
      directPermissions: { code: string; scope: string }[];
      expectedVersion: number;
    },
  ) => Promise<AccessUser>;
  roles: (input: { page: number; perPage: number }) => Promise<{
    list: AccessRoleSummary[];
    page: { total: number; totalPage: number };
  }>;
  catalog: () => Promise<AccessCatalogItem[]>;
};

async function loadAllRoles(api: Api) {
  const first = await api.roles({ page: 1, perPage: 100 });
  const rest = await Promise.all(
    Array.from({ length: Math.max(0, first.page.totalPage - 1) }, (_, index) =>
      api.roles({ page: index + 2, perPage: 100 }),
    ),
  );
  const byID = new Map(first.list.map((role) => [role.id, role]));
  for (const page of rest) {
    for (const role of page.list) byID.set(role.id, role);
  }
  return [...byID.values()];
}

export function AccessStaffPage({ api }: { api: Api }) {
  const client = useQueryClient();
  const [page, setPage] = React.useState(1);
  const [provisioning, setProvisioning] = React.useState<AccessEmployee | null>(
    null,
  );
  const [editing, setEditing] = React.useState<AccessEmployee | null>(null);
  const [loginIdentifier, setLoginIdentifier] = React.useState("");
  const [roleIds, setRoleIds] = React.useState<number[]>([]);
  const [direct, setDirect] = React.useState<{ code: string; scope: string }[]>(
    [],
  );
  const [temporaryPassword, setTemporaryPassword] = React.useState("");

  const employees = useQuery({
    queryKey: ["access-employees", page],
    queryFn: () => api.employees({ page, perPage: 50 }),
  });
  const roles = useQuery({
    queryKey: ["access-role-options"],
    queryFn: () => loadAllRoles(api),
  });
  const catalog = useQuery({
    queryKey: ["access-catalog-options"],
    queryFn: api.catalog,
  });
  const detail = useQuery({
    queryKey: ["access-user", editing?.account?.userId],
    queryFn: () => api.user(editing!.account!.userId),
    enabled: editing?.account !== null && editing !== null,
  });

  React.useEffect(() => {
    if (detail.data) {
      setRoleIds(detail.data.roles.map((role) => role.id));
      setDirect(detail.data.directPermissions);
    }
  }, [detail.data]);

  const refreshEmployees = () =>
    client.invalidateQueries({ queryKey: ["access-employees"] });

  const provision = useMutation({
    mutationFn: () => {
      if (!provisioning) throw new Error("未选择员工");
      return api.provisionEmployeeAccount(provisioning.id, {
        loginIdentifier,
        roleIds,
        directPermissions: direct,
      });
    },
    onSuccess: (result) => {
      setProvisioning(null);
      setRoleIds([]);
      setDirect([]);
      setTemporaryPassword(result.temporaryPassword ?? "");
      void refreshEmployees();
    },
  });
  const statusMutation = useMutation({
    mutationFn: (input: { employee: AccessEmployee; status: 1 | 2 }) =>
      api.updateEmployeeAccountStatus(input.employee.id, input.status),
    onSuccess: () => void refreshEmployees(),
  });
  const resetPassword = useMutation({
    mutationFn: (employee: AccessEmployee) =>
      api.resetEmployeePassword(employee.id),
    onSuccess: (result) => {
      setTemporaryPassword(result.temporaryPassword ?? "");
      void refreshEmployees();
    },
  });

  const canEdit =
    detail.isSuccess && detail.data !== undefined && roles.isSuccess && catalog.isSuccess;
  const save = useMutation({
    mutationFn: () => {
      if (!editing?.account || !detail.data || !canEdit) {
        throw new Error("员工账号详情尚未加载完成");
      }
      return api.replaceUser(editing.account.userId, {
        roleIds,
        directPermissions: direct,
        expectedVersion: detail.data.version,
      });
    },
    onSuccess: () => {
      setEditing(null);
      setRoleIds([]);
      setDirect([]);
      void refreshEmployees();
    },
  });
  const saveError = save.error as { status?: number } | null;

  return (
    <Phase35PageShell
      title="员工账号与权限"
      description="从已同步的企业微信员工开通登录账号，并管理停用、密码和页面权限"
    >
      <div className="phase35-page">
        <section className="phase35-card phase35-table-card">
          {employees.isLoading ? <p>正在加载员工…</p> : null}
          {employees.isError ? (
            <p role="alert">
              员工列表加载失败
              <button type="button" onClick={() => void employees.refetch()}>
                重试
              </button>
            </p>
          ) : null}
          <div className="dashboard-table-scroll" data-testid="employee-account-table">
            <table>
              <thead>
                <tr>
                  <th>企业微信员工</th>
                  <th>企微状态</th>
                  <th>登录账号</th>
                  <th>账号状态</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {(employees.data?.list ?? []).map((employee) => (
                  <tr key={employee.id}>
                    <td>
                      <strong>{employee.name}</strong>
                      <br />
                      <small>{employee.mobile || employee.wxUserId || "未提供手机"}</small>
                    </td>
                    <td>{employee.status === 1 ? "已激活" : "未激活或已离职"}</td>
                    <td>{employee.account?.loginIdentifier ?? "尚未开通"}</td>
                    <td>
                      {!employee.account
                        ? "未开通"
                        : employee.account.status === 1
                          ? employee.account.mustRotatePassword
                            ? "待首次修改密码"
                            : "正常"
                          : "已停用"}
                    </td>
                    <td>
                      {!employee.account ? (
                        <button
                          type="button"
                          onClick={() => {
                            provision.reset();
                            setRoleIds([]);
                            setDirect([]);
                            setLoginIdentifier(employee.mobile);
                            setProvisioning(employee);
                          }}
                        >
                          开通账号
                        </button>
                      ) : (
                        <div className="dashboard-actions-inline">
                          <button
                            type="button"
                            onClick={() => {
                              save.reset();
                              setRoleIds([]);
                              setDirect([]);
                              setEditing(employee);
                            }}
                          >
                            权限设置
                          </button>
                          <ConfirmAction
                            title={`${employee.account.status === 1 ? "停用" : "启用"} ${employee.name} 的登录账号？`}
                            onConfirm={() =>
                              statusMutation.mutate({
                                employee,
                                status: employee.account!.status === 1 ? 2 : 1,
                              })
                            }
                          >
                            <button type="button">
                              {employee.account.status === 1 ? "停用" : "启用"}
                            </button>
                          </ConfirmAction>
                          <ConfirmAction
                            title={`重置 ${employee.name} 的密码？`}
                            description="旧会话会立即失效，新密码只展示一次。"
                            onConfirm={() => resetPassword.mutate(employee)}
                          >
                            <button type="button" disabled={employee.account.status !== 1}>
                              重置密码
                            </button>
                          </ConfirmAction>
                        </div>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {!employees.isLoading && (employees.data?.list.length ?? 0) === 0 ? (
            <p>尚未同步到企业微信员工，请先在企业设置中执行员工同步。</p>
          ) : null}
          <DashboardPagination
            page={page}
            pageSize={50}
            total={employees.data?.page.total ?? 0}
            onPageChange={setPage}
          />
          {statusMutation.isError || resetPassword.isError ? (
            <p role="alert">账号操作失败，请刷新后重试。</p>
          ) : null}
        </section>

        {provisioning ? (
          <DashboardDialog
            open
            title={`为 ${provisioning.name} 开通账号`}
            width={920}
            onCancel={() => {
              setProvisioning(null);
              setRoleIds([]);
              setDirect([]);
              provision.reset();
            }}
            footer={
              <>
                <button type="button" onClick={() => {
                  setProvisioning(null);
                  setRoleIds([]);
                  setDirect([]);
                  provision.reset();
                }}>
                  取消
                </button>
                <ConfirmAction
                  title={`确认开通 ${provisioning.name} 的 Dashboard 账号？`}
                  description={`将授予 ${roleIds.length} 个角色和 ${direct.length} 项直接权限；系统会生成一次性初始密码。`}
                  onConfirm={() => provision.mutate()}
                >
                  <button
                    type="button"
                    disabled={!/^\d{11}$/.test(loginIdentifier) || !roles.isSuccess || !catalog.isSuccess || provision.isPending}
                  >
                    确认开通
                  </button>
                </ConfirmAction>
              </>
            }
          >
            <div className="access-provision-intro">
              <div className="access-provision-avatar" aria-hidden="true">
                {provisioning.name.slice(0, 1)}
              </div>
              <div>
                <strong>{provisioning.name}</strong>
                <span>{provisioning.mobile || provisioning.wxUserId}</span>
              </div>
              <small>登录后仅能访问下方已选角色与直授权限</small>
            </div>
            <div className="access-provision-basics">
              <label className="phase35-field">
                <span>登录手机号</span>
                <input
                  aria-label="员工登录手机号"
                  inputMode="numeric"
                  maxLength={11}
                  value={loginIdentifier}
                  onChange={(event) => setLoginIdentifier(event.target.value.replace(/\D/g, ""))}
                />
                <small>员工使用该手机号登录，首次登录必须修改初始密码。</small>
              </label>
              <div className="access-provision-selection-summary" role="status">
                <span>当前授权</span>
                <strong>{roleIds.length} 个角色 · {direct.length} 项直接权限</strong>
              </div>
            </div>
            <RoleSelector roles={roles.data ?? []} value={roleIds} onChange={setRoleIds} />
            <AccessPermissionSelector
              ariaLabel="员工直接权限"
              catalog={catalog.data ?? []}
              value={direct}
              onChange={setDirect}
            />
            {provision.isError ? <p role="alert">账号开通失败，手机号可能已被使用。</p> : null}
          </DashboardDialog>
        ) : null}

        {editing ? (
          <DashboardDialog
            open
            title={`设置 ${editing.name} 的权限`}
            width={820}
            onCancel={() => {
              setEditing(null);
              setRoleIds([]);
              setDirect([]);
              save.reset();
            }}
            footer={
              <>
                <button type="button" onClick={() => setEditing(null)}>
                  取消
                </button>
                <ConfirmAction
                  title={`保存 ${editing.name} 的 ${roleIds.length} 个角色与 ${direct.length} 项直接权限？`}
                  onConfirm={() => save.mutate()}
                >
                  <button type="button" disabled={!canEdit || save.isPending}>
                    保存权限
                  </button>
                </ConfirmAction>
              </>
            }
          >
            <div className="access-editor-summary" role="status">
              <strong>{editing.name}</strong>
              <span>
                已选 {roleIds.length} 个角色 · {direct.length} 项直接权限
              </span>
            </div>
            {detail.isLoading ? <p>正在加载账号权限…</p> : null}
            {detail.isError ? (
              <p role="alert">
                账号权限加载失败
                <button type="button" onClick={() => void detail.refetch()}>
                  重试
                </button>
              </p>
            ) : null}
            {canEdit ? (
              <>
                <RoleSelector roles={roles.data ?? []} value={roleIds} onChange={setRoleIds} />
                <AccessPermissionSelector
                  ariaLabel="直接权限"
                  catalog={catalog.data ?? []}
                  value={direct}
                  onChange={setDirect}
                />
              </>
            ) : null}
            {save.error ? (
              <p role="alert">
                {saveError?.status === 409
                  ? "权限版本已变化，表单已保留，请刷新后重试。"
                  : "权限保存失败，请稍后重试。"}
              </p>
            ) : null}
          </DashboardDialog>
        ) : null}

        {temporaryPassword ? (
          <DashboardDialog
            open
            title="一次性初始密码"
            onCancel={() => setTemporaryPassword("")}
            footer={
              <button type="button" onClick={() => setTemporaryPassword("")}>
                我已保存
              </button>
            }
          >
            <p>请立即安全地交给员工。关闭后系统不会再次显示，首次登录必须修改密码。</p>
            <div className="access-temporary-password">
              <code>{temporaryPassword}</code>
              <button type="button" onClick={() => void navigator.clipboard.writeText(temporaryPassword)}>
                复制
              </button>
            </div>
          </DashboardDialog>
        ) : null}
      </div>
    </Phase35PageShell>
  );
}

function RoleSelector({
  roles,
  value,
  onChange,
}: {
  roles: AccessRoleSummary[];
  value: number[];
  onChange: (value: number[]) => void;
}) {
  return (
    <fieldset className="access-role-selector">
      <legend>角色（可多选）</legend>
      <div className="access-role-grid">
        {roles.map((role) => (
          <label
            className={`access-role-option${value.includes(role.id) ? " access-role-option--selected" : ""}`}
            key={role.id}
          >
            <input
              aria-label={`${role.name}（${role.status === 1 ? "启用" : "停用"}）`}
              type="checkbox"
              disabled={role.status !== 1}
              checked={value.includes(role.id)}
              onChange={() =>
                onChange(
                  value.includes(role.id)
                    ? value.filter((id) => id !== role.id)
                    : [...value, role.id],
                )
              }
            />
            <span>
              <strong>{role.name}</strong>
              <small>{role.status === 1 ? "启用" : "停用"}</small>
            </span>
          </label>
        ))}
      </div>
    </fieldset>
  );
}
