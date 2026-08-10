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

type Api = {
  users: (input: {
    page: number;
    perPage: number;
  }) => Promise<{
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
  roles: (input: {
    page: number;
    perPage: number;
  }) => Promise<{ list: AccessRoleSummary[] }>;
  catalog: () => Promise<AccessCatalogItem[]>;
};

export function AccessStaffPage({ api }: { api: Api }) {
  const client = useQueryClient();
  const [selected, setSelected] = React.useState<AccessUserSummary | null>(
    null,
  );
  const [roleIds, setRoleIds] = React.useState<number[]>([]);
  const [direct, setDirect] = React.useState<{ code: string; scope: string }[]>(
    [],
  );
  const users = useQuery({
    queryKey: ["access-users"],
    queryFn: () => api.users({ page: 1, perPage: 50 }),
  });
  const roles = useQuery({
    queryKey: ["access-role-options"],
    queryFn: () => api.roles({ page: 1, perPage: 200 }),
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
  const save = useMutation({
    mutationFn: () =>
      api.replaceUser(selected!.id, {
        roleIds,
        directPermissions: direct,
        expectedVersion: detail.data?.version ?? selected!.version,
      }),
    onSuccess: () => {
      setSelected(null);
      void client.invalidateQueries({ queryKey: ["access-users"] });
    },
  });
  const saveError = save.error as { status?: number } | null;
  const summary = selected
    ? `将更新员工“${selected.name}”的 ${roleIds.length} 个角色与 ${direct.length} 项直接权限`
    : "";
  return (
    <Phase35PageShell title="员工权限" description="多角色、直接权限与继承来源">
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
                      <button type="button" onClick={() => setSelected(item)}>
                        编辑权限
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p>共 {users.data?.page.total ?? 0} 名员工</p>
        </section>
        {selected && (
          <DashboardDialog
            open
            title={`编辑 ${selected.name} 权限`}
            onCancel={() => setSelected(null)}
            confirmDisabled={detail.isLoading || save.isPending}
            footer={
              <>
                <button type="button" onClick={() => setSelected(null)}>
                  取消
                </button>
                <ConfirmAction title={summary} onConfirm={() => save.mutate()}>
                  <button
                    type="button"
                    disabled={detail.isLoading || save.isPending}
                  >
                    保存权限
                  </button>
                </ConfirmAction>
              </>
            }
          >
            <fieldset>
              <legend>角色（可多选）</legend>
              {detail.isLoading ? (
                <p>正在加载详情…</p>
              ) : (
                (roles.data?.list ?? []).map((role) => (
                  <label key={role.id}>
                    <input
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
                    {role.name}（{role.status === 1 ? "启用" : "停用"}）
                  </label>
                ))
              )}
            </fieldset>
            <fieldset>
              <legend>直接权限</legend>
              {(catalog.data ?? [])
                .filter((item) => !item.superadminOnly)
                .map((item) => {
                  const found = direct.find(
                    (permission) => permission.code === item.code,
                  );
                  return (
                    <label key={item.code}>
                      <input
                        type="checkbox"
                        checked={found !== undefined}
                        onChange={() =>
                          setDirect((current) =>
                            found
                              ? current.filter(
                                  (permission) => permission.code !== item.code,
                                )
                              : [
                                  ...current,
                                  { code: item.code, scope: "self" },
                                ],
                          )
                        }
                      />
                      {item.name}
                      <select
                        aria-label={`${item.name} 数据范围`}
                        value={found?.scope ?? "self"}
                        disabled={!found}
                        onChange={(event) =>
                          setDirect((current) =>
                            current.map((permission) =>
                              permission.code === item.code
                                ? { ...permission, scope: event.target.value }
                                : permission,
                            ),
                          )
                        }
                      >
                        <option value="self">本人</option>
                        <option value="department">部门</option>
                        <option value="tenant">全企业</option>
                      </select>
                    </label>
                  );
                })}
            </fieldset>
            <h3>继承权限（只读）</h3>
            {(detail.data?.inheritedPermissions ?? []).map((permission) => (
              <p key={permission.code}>
                {permission.name} ·{" "}
                {permission.sources
                  .map((source) => `${source.name}/${source.scope}`)
                  .join("、")}
              </p>
            ))}
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
