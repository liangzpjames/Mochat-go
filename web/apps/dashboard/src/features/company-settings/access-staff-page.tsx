import * as React from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { DashboardDialog } from "../../components/dashboard-dialog";
import { ConfirmAction } from "../../components/confirm-action";
import { Phase35PageShell } from "../phase35/components/phase35-page-shell";
import type { AccessCatalogItem } from "../access/access-api";
import type {
  AccessRoleSummary,
  AccessUser,
  AccessUserSummary,
} from "../access/access-admin-api";
import { AccessPermissionSelector } from "./access-permission-selector";

type Api = {
  users: (input: { page: number; perPage: number }) => Promise<{
    list: AccessUserSummary[];
    page: { total: number; totalPage: number };
  }>;
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

export function AccessStaffPage({ api }: { api: Api }) {
  const client = useQueryClient();
  const [selected, setSelected] = React.useState<AccessUserSummary | null>(
    null,
  );
  const [userPage, setUserPage] = React.useState(1);
  const [roleIds, setRoleIds] = React.useState<number[]>([]);
  const [direct, setDirect] = React.useState<{ code: string; scope: string }[]>(
    [],
  );
  const users = useQuery({
    queryKey: ["access-users", userPage],
    queryFn: () => api.users({ page: userPage, perPage: 50 }),
  });
  const roles = useQuery({
    queryKey: ["access-role-options"],
    queryFn: async () => {
      const first = await api.roles({ page: 1, perPage: 100 });
      const pages = first.page.totalPage;
      const rest = await Promise.all(
        Array.from({ length: Math.max(0, pages - 1) }, (_, index) =>
          api.roles({ page: index + 2, perPage: 100 }),
        ),
      );
      const byID = new Map(first.list.map((role) => [role.id, role]));
      for (const page of rest) {
        for (const role of page.list) byID.set(role.id, role);
      }
      return [...byID.values()];
    },
  });
  const catalog = useQuery({
    queryKey: ["access-catalog-options"],
    queryFn: api.catalog,
  });
  const detail = useQuery({
    queryKey: ["access-user", selected?.id],
    queryFn: () => api.user(selected!.id),
    enabled: selected !== null,
  });
  React.useEffect(() => {
    if (detail.data) {
      setRoleIds(detail.data.roles.map((role) => role.id));
      setDirect(detail.data.directPermissions);
    }
  }, [detail.data]);
  const canEdit =
    detail.isSuccess &&
    detail.data !== undefined &&
    roles.isSuccess &&
    catalog.isSuccess;
  const save = useMutation({
    mutationFn: () => {
      if (!selected || !detail.data || !canEdit) {
        throw new Error("员工详情尚未加载完成");
      }
      return api.replaceUser(selected.id, {
        roleIds,
        directPermissions: direct,
        expectedVersion: detail.data.version,
      });
    },
    onSuccess: () => {
      setSelected(null);
      setRoleIds([]);
      setDirect([]);
      void client.invalidateQueries({ queryKey: ["access-users", userPage] });
    },
  });
  const saveError = save.error as { status?: number } | null;
  const summary = selected
    ? `将更新员工“${selected.name}”的 ${roleIds.length} 个角色与 ${direct.length} 项直接权限`
    : "";
  return (
    <Phase35PageShell title="员工权限" description="为员工配置角色与直接页面权限">
      <div className="phase35-page">
        <section className="phase35-card phase35-table-card">
          <div className="dashboard-table-scroll">
            <table>
              <thead>
                <tr>
                  <th>员工</th>
                  <th>状态</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {(users.data?.list ?? []).map((item) => (
                  <tr key={item.id}>
                    <td>
                      {item.name}
                      <br />
                      <small>{item.phone}</small>
                    </td>
                    <td>{item.status === 1 ? "正常" : "停用"}</td>
                    <td>
                      <button
                        type="button"
                        onClick={() => {
                          setRoleIds([]);
                          setDirect([]);
                          save.reset();
                          setSelected(item);
                        }}
                      >
                        编辑权限
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p>共 {users.data?.page.total ?? 0} 名员工</p>
          <div className="dashboard-pagination">
            <button
              type="button"
              disabled={userPage <= 1}
              onClick={() => setUserPage((page) => page - 1)}
            >
              上一页
            </button>
            <span>
              第 {userPage}/{users.data?.page.totalPage ?? 1} 页
            </span>
            <button
              type="button"
              disabled={userPage >= (users.data?.page.totalPage ?? 1)}
              onClick={() => setUserPage((page) => page + 1)}
            >
              下一页
            </button>
          </div>
        </section>
        {selected && (
          <DashboardDialog
            open
            title={`编辑 ${selected.name} 权限`}
            width={820}
            onCancel={() => {
              setSelected(null);
              setRoleIds([]);
              setDirect([]);
              save.reset();
            }}
            confirmDisabled={!canEdit || save.isPending}
            footer={
              <>
                <button
                  type="button"
                  onClick={() => {
                    setSelected(null);
                    setRoleIds([]);
                    setDirect([]);
                    save.reset();
                  }}
                >
                  取消
                </button>
                <ConfirmAction title={summary} onConfirm={() => save.mutate()}>
                  <button type="button" disabled={!canEdit || save.isPending}>
                    保存权限
                  </button>
                </ConfirmAction>
              </>
            }
          >
            <div className="access-editor-summary" role="status">
              <strong>{selected.name}</strong>
              <span>
                已选 {roleIds.length} 个角色 · {direct.length} 项直接权限
              </span>
            </div>
            <fieldset className="access-role-selector">
              <legend>角色（可多选）</legend>
              {detail.isLoading ? (
                <p>正在加载详情…</p>
              ) : detail.isError ? (
                <p role="alert">
                  员工详情加载失败，请重试
                  <button type="button" onClick={() => void detail.refetch()}>
                    重试
                  </button>
                </p>
              ) : !canEdit ? (
                <p>正在准备可编辑权限数据…</p>
              ) : (
                <div className="access-role-grid">
                  {(roles.data ?? []).map((role) => (
                    <label
                      className={`access-role-option${roleIds.includes(role.id) ? " access-role-option--selected" : ""}`}
                      key={role.id}
                    >
                      <input
                        aria-label={`${role.name}（${role.status === 1 ? "启用" : "停用"}）`}
                        type="checkbox"
                      disabled={role.status !== 1}
                      checked={roleIds.includes(role.id)}
                      onChange={() =>
                        setRoleIds((current) =>
                          current.includes(role.id)
                            ? current.filter((id) => id !== role.id)
                            : [...current, role.id],
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
              )}
            </fieldset>
            <AccessPermissionSelector
              ariaLabel="直接权限"
              catalog={catalog.data ?? []}
              value={direct}
              onChange={setDirect}
            />
            {save.error && (
              <p role="alert">
                {saveError?.status === 409
                  ? "版本冲突（409），表单已保留，请刷新后重试"
                  : "保存失败，请稍后重试"}
              </p>
            )}
          </DashboardDialog>
        )}
      </div>
    </Phase35PageShell>
  );
}
