import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { ConfirmAction } from "../../components/confirm-action";
import { DashboardDialog } from "../../components/dashboard-dialog";
import { DashboardPagination } from "../../components/dashboard-pagination";
import { Phase35PageShell } from "../phase35/components/phase35-page-shell";
import { Phase35DataState } from "../phase35/components/data-state";
import {
  createUserAdminApi,
  type UserItem,
  type UserWrite,
} from "../user-admin/user-admin-api";
import type { DashboardAccessAdminApi } from "../access/access-admin-api";
import { AccessStaffPage as NewAccessStaffPage } from "./access-staff-page";

type UserAdminApi = ReturnType<typeof createUserAdminApi>;

function LegacyCompanyStaffPage({ api }: { api: UserAdminApi }) {
  const queryClient = useQueryClient();
  const [phoneFilter, setPhoneFilter] = useState("");
  const [appliedPhone, setAppliedPhone] = useState("");
  const [status, setStatus] = useState<number | "">("");
  const [page, setPage] = useState(1);
  const [editing, setEditing] = useState<UserItem | null>(null);
  const [creating, setCreating] = useState(false);
  const [userName, setUserName] = useState("");
  const [formPhone, setFormPhone] = useState("");
  const [roleId, setRoleId] = useState(1);
  const [formStatus, setFormStatus] = useState(1);
  const [error, setError] = useState("");
  const triggerRef = useRef<HTMLElement | null>(null);

  const query = useQuery({
    queryKey: ["company-staff", appliedPhone, status, page],
    queryFn: () => api.list({ phone: appliedPhone, status, page, perPage: 20 }),
  });
  const roles = useQuery({
    queryKey: ["role-options"],
    queryFn: () => api.roles(),
  });
  const items = query.data?.list ?? [];
  const total = query.data?.page?.total ?? 0;
  const totalPage = query.data?.page?.totalPage ?? 1;

  const save = useMutation({
    mutationFn: () =>
      editing
        ? api.update(editing.userId, {
            userName: userName.trim(),
            phone: formPhone.trim(),
            gender: 1,
            roleId,
            status: formStatus,
          } satisfies UserWrite)
        : api.create({
            userName: userName.trim(),
            phone: formPhone.trim(),
            gender: 1,
            roleId,
            status: formStatus,
            password: "P36-ACCEPT-Pass1",
            confirmPass: "P36-ACCEPT-Pass1",
          }),
    onSuccess: () => {
      setEditing(null);
      setCreating(false);
      setError("");
      void queryClient.invalidateQueries({ queryKey: ["company-staff"] });
    },
    onError: (err) => setError(err instanceof Error ? err.message : "保存失败"),
  });
  const toggleStatus = useMutation({
    mutationFn: (input: { ids: number[]; next: number }) =>
      api.updateStatus(input.ids, input.next),
    onSuccess: () =>
      void queryClient.invalidateQueries({ queryKey: ["company-staff"] }),
  });
  const openCreate = () => {
    setCreating(true);
    setEditing(null);
    setUserName("");
    setFormPhone("");
    setRoleId(roles.data?.[0]?.roleId ?? 1);
    setFormStatus(1);
    setError("");
  };
  const openEdit = (item: UserItem) => {
    setEditing(item);
    setCreating(false);
    setUserName(item.userName);
    setFormPhone(item.phone);
    setRoleId(item.roleId);
    setFormStatus(item.status);
    setError("");
  };
  const valid = Boolean(userName.trim() && /^1\d{10}$/.test(formPhone.trim()));
  const closeEditor = () => {
    setEditing(null);
    setCreating(false);
    setError("");
  };

  return (
    <Phase35PageShell
      title="员工权限"
      description="员工账号、启用/停用状态与角色分配"
      actions={
        <button
          type="button"
          onClick={(event) => {
            triggerRef.current = event.currentTarget;
            openCreate();
          }}
        >
          新增员工
        </button>
      }
    >
      <div className="phase35-page">
        <section className="phase35-card phase35-filter-card">
          <form
            className="dashboard-filter-bar"
            onSubmit={(event) => {
              event.preventDefault();
              setPage(1);
              setAppliedPhone(phoneFilter.trim());
            }}
          >
            <input
              aria-label="手机号筛选"
              value={phoneFilter}
              onChange={(event) => setPhoneFilter(event.target.value)}
              placeholder="按手机号筛选"
            />
            <select
              aria-label="状态"
              value={status}
              onChange={(event) =>
                setStatus(
                  event.target.value === "" ? "" : Number(event.target.value),
                )
              }
            >
              <option value="">全部状态</option>
              <option value={1}>启用</option>
              <option value={0}>停用</option>
            </select>
            <button type="submit">查询</button>
            <button
              type="button"
              onClick={() => {
                setPhoneFilter("");
                setAppliedPhone("");
                setStatus("");
              }}
            >
              重置
            </button>
          </form>
        </section>

        <section className="phase35-kpis" aria-label="员工指标">
          <article className="phase35-kpi phase35-kpi-primary">
            <span>员工总数</span>
            <strong>{total}</strong>
            <small>筛选条件下员工数量</small>
          </article>
          <article className="phase35-kpi phase35-kpi-green">
            <span>正常</span>
            <strong>{query.data?.normalNum ?? 0}</strong>
            <small>状态为正常的员工</small>
          </article>
          <article className="phase35-kpi phase35-kpi-red">
            <span>停用</span>
            <strong>{query.data?.disableNum ?? 0}</strong>
            <small>状态为停用的员工</small>
          </article>
        </section>

        <section className="phase35-card phase35-table-card">
          <header className="phase35-card-header">
            <div>
              <h2>员工列表</h2>
              <p>启用/停用与角色分配</p>
            </div>
            <span className="phase35-chip">
              共 {total} 条，第 {page}/{totalPage} 页
            </span>
          </header>
          <Phase35DataState
            loading={query.isLoading}
            error={query.isError}
            empty={!items.length}
            emptyContent={<p className="phase35-empty">暂无员工</p>}
            onRetry={() => void query.refetch()}
          >
            <div className="phase35-table">
              <table>
                <thead>
                  <tr>
                    <th>姓名</th>
                    <th>手机号</th>
                    <th>角色</th>
                    <th>状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((item) => (
                    <tr key={item.userId}>
                      <td>{item.userName}</td>
                      <td>{item.phone}</td>
                      <td>{item.roleName}</td>
                      <td>{item.statusText}</td>
                      <td>
                        <button
                          type="button"
                          onClick={(event) => {
                            triggerRef.current = event.currentTarget;
                            openEdit(item);
                          }}
                        >
                          编辑
                        </button>
                        <ConfirmAction
                          title={
                            item.status === 1
                              ? `确认停用员工“${item.userName}”？`
                              : `确认启用员工“${item.userName}”？`
                          }
                          description={
                            item.status === 1
                              ? "停用后该员工将无法登录 Dashboard。"
                              : "启用后该员工将恢复登录权限。"
                          }
                          onConfirm={() =>
                            toggleStatus.mutate({
                              ids: [item.userId],
                              next: item.status === 1 ? 0 : 1,
                            })
                          }
                        >
                          <button type="button">
                            {item.status === 1
                              ? `停用 ${item.userName}`
                              : `启用 ${item.userName}`}
                          </button>
                        </ConfirmAction>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <DashboardPagination page={page} pageSize={20} total={total} onPageChange={setPage} />
          </Phase35DataState>
        </section>

        <DashboardDialog
          open={creating || editing !== null}
          title={editing ? "编辑员工" : "新增员工"}
          triggerRef={triggerRef}
          confirmDisabled={!valid}
          confirmLoading={save.isPending}
          onCancel={closeEditor}
          onConfirm={() => save.mutate()}
        >
          {error && (
            <p role="alert" className="phase35-limits">
              {error}
            </p>
          )}
          <form
            onSubmit={(event) => {
              event.preventDefault();
              if (valid) save.mutate();
            }}
          >
            <label>
              姓名
              <input
                value={userName}
                onChange={(event) => setUserName(event.target.value)}
              />
            </label>
            <label>
              手机号
              <input
                aria-label="员工手机号"
                value={formPhone}
                onChange={(event) => setFormPhone(event.target.value)}
              />
            </label>
            <label>
              角色
              <select
                value={roleId}
                onChange={(event) => setRoleId(Number(event.target.value))}
              >
                {(roles.data ?? []).map((role) => (
                  <option key={role.roleId} value={role.roleId}>
                    {role.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              状态
              <select
                value={formStatus}
                onChange={(event) => setFormStatus(Number(event.target.value))}
              >
                <option value={1}>启用</option>
                <option value={0}>停用</option>
              </select>
            </label>
          </form>
        </DashboardDialog>
      </div>
    </Phase35PageShell>
  );
}

type StaffPageApi = UserAdminApi | DashboardAccessAdminApi;

function isDashboardAccessApi(
  api: StaffPageApi,
): api is DashboardAccessAdminApi {
  return "users" in api;
}

export function CompanyStaffPage({ api }: { api: StaffPageApi }) {
  return isDashboardAccessApi(api) ? (
    <NewAccessStaffPage api={api} />
  ) : (
    <LegacyCompanyStaffPage api={api} />
  );
}
