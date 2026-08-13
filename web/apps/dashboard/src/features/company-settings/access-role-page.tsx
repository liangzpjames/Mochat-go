import * as React from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { DashboardDialog } from "../../components/dashboard-dialog";
import { ConfirmAction } from "../../components/confirm-action";
import { DashboardPagination } from "../../components/dashboard-pagination";
import { Phase35PageShell } from "../phase35/components/phase35-page-shell";
import type { AccessCatalogItem } from "../access/access-api";
import type { AccessRole } from "../access/access-admin-api";
import { AccessPermissionSelector } from "./access-permission-selector";

type Api = {
  roles: (input: { page: number; perPage: number }) => Promise<{
    list: AccessRole[];
    page: { total: number; totalPage: number };
  }>;
  catalog: () => Promise<AccessCatalogItem[]>;
  createRole: (input: {
    name: string;
    remark: string;
    status: number;
    permissions: { code: string; scope: string }[];
  }) => Promise<AccessRole>;
  updateRole: (
    id: number,
    input: {
      name: string;
      remark: string;
      permissions: { code: string; scope: string }[];
      expectedVersion: number;
    },
  ) => Promise<AccessRole>;
  updateRoleStatus: (
    id: number,
    input: { status: number; expectedVersion: number },
  ) => Promise<AccessRole>;
  deleteRole: (id: number, expectedVersion: number) => Promise<unknown>;
};

export function AccessRolePage({ api }: { api: Api }) {
  const client = useQueryClient();
  const [editing, setEditing] = React.useState<AccessRole | null>(null);
  const [name, setName] = React.useState("");
  const [remark, setRemark] = React.useState("");
  const [codes, setCodes] = React.useState<{ code: string; scope: string }[]>(
    [],
  );
  const [error, setError] = React.useState("");
  const [page, setPage] = React.useState(1);
  const roles = useQuery({
    queryKey: ["access-roles", page],
    queryFn: () => api.roles({ page, perPage: 50 }),
  });
  const catalog = useQuery({
    queryKey: ["access-role-catalog"],
    queryFn: api.catalog,
  });
  const save = useMutation({
    mutationFn: () =>
      editing?.id
        ? api.updateRole(editing.id, {
            name: name.trim(),
            remark: remark.trim(),
            permissions: codes,
            expectedVersion: editing.version,
          })
        : api.createRole({
            name: name.trim(),
            remark: remark.trim(),
            status: 1,
            permissions: codes,
          }),
    onSuccess: () => {
      setEditing(null);
      void client.invalidateQueries({ queryKey: ["access-roles"] });
    },
    onError: (e) =>
      setError(
        e instanceof Error &&
          "status" in e &&
          (e as { status?: number }).status === 409
          ? "版本冲突（409），表单已保留，请刷新后重试"
          : "保存失败",
      ),
  });
  const toggle = useMutation({
    mutationFn: (role: AccessRole) =>
      api.updateRoleStatus(role.id, {
        status: role.status === 1 ? 2 : 1,
        expectedVersion: role.version,
      }),
    onSuccess: () =>
      void client.invalidateQueries({ queryKey: ["access-roles"] }),
  });
  const remove = useMutation({
    mutationFn: (role: AccessRole) => api.deleteRole(role.id, role.version),
    onSuccess: () =>
      void client.invalidateQueries({ queryKey: ["access-roles"] }),
    onError: (e) =>
      setError(
        e instanceof Error &&
          "status" in e &&
          (e as { status?: number }).status === 409
          ? "删除失败（409）：角色仍有成员或版本已变化"
          : "删除失败",
      ),
  });
  const open = (role: AccessRole) => {
    setEditing(role);
    setName(role.name);
    setRemark(role.remark);
    setCodes(role.permissions);
    setError("");
  };
  const summary = editing
    ? `将${editing.id ? "更新" : "创建"}角色“${name || editing.name}”，授予 ${codes.length} 项页面权限`
    : "";
  return (
    <Phase35PageShell
      title="角色管理"
      description="角色 CRUD、权限范围、启停与成员约束"
      actions={
        <button
          type="button"
          onClick={() => {
            setEditing({
              id: 0,
              name: "",
              remark: "",
              status: 1,
              isSystem: false,
              memberCount: 0,
              permissions: [],
              version: 1,
            });
            setName("");
            setRemark("");
            setCodes([]);
          }}
        >
          新建角色
        </button>
      }
    >
      <div className="phase35-page">
        <section className="phase35-card phase35-table-card">
          <div className="dashboard-table-scroll">
            <table>
              <thead>
                <tr>
                  <th>角色</th>
                  <th>状态</th>
                  <th>成员</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {(roles.data?.list ?? []).map((role) => (
                  <tr key={role.id}>
                    <td>
                      {role.name}
                      <br />
                      <small>{role.remark}</small>
                    </td>
                    <td>{role.status === 1 ? "启用" : "停用"}</td>
                    <td>{role.memberCount}</td>
                    <td>
                      {role.isSystem ? (
                        <span>系统角色（不可修改）</span>
                      ) : (
                        <>
                          <button type="button" onClick={() => open(role)}>
                            编辑
                          </button>
                          <ConfirmAction
                            title={`${role.status === 1 ? "停用" : "启用"}角色“${role.name}”？`}
                            onConfirm={() => toggle.mutate(role)}
                          >
                            <button type="button">
                              {role.status === 1 ? "停用" : "启用"}
                            </button>
                          </ConfirmAction>
                          <ConfirmAction
                            title={`确认删除角色“${role.name}”？`}
                            onConfirm={() => remove.mutate(role)}
                          >
                            <button type="button">删除</button>
                          </ConfirmAction>
                        </>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {error && <p role="alert">{error}</p>}
          <DashboardPagination
            page={page}
            pageSize={50}
            total={roles.data?.page.total ?? 0}
            onPageChange={setPage}
          />
        </section>
        {editing && (
          <DashboardDialog
            open
            title={editing.id ? "编辑角色" : "新建角色"}
            width={820}
            onCancel={() => setEditing(null)}
            confirmDisabled={save.isPending}
            footer={
              <>
                <button type="button" onClick={() => setEditing(null)}>
                  取消
                </button>
                <ConfirmAction title={summary} onConfirm={() => save.mutate()}>
                  <button type="button" disabled={save.isPending}>
                    保存
                  </button>
                </ConfirmAction>
              </>
            }
          >
            <div className="access-editor-summary" role="status">
              <strong>{editing.id ? "编辑角色" : "新建角色"}</strong>
              <span>{codes.length} 项页面权限</span>
            </div>
            <div className="access-role-basics">
              <label>
                角色名称
                <input
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                />
              </label>
              <label>
                备注
                <input
                  value={remark}
                  onChange={(event) => setRemark(event.target.value)}
                />
              </label>
            </div>
            <AccessPermissionSelector
              ariaLabel="页面权限与数据范围"
              catalog={catalog.data ?? []}
              value={codes}
              onChange={setCodes}
            />
          </DashboardDialog>
        )}
      </div>
    </Phase35PageShell>
  );
}
