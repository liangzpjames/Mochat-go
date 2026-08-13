import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import * as React from "react";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AccessStaffPage } from "./access-staff-page";
import { AccessRolePage } from "./access-role-page";
import { AccessPermissionSelector } from "./access-permission-selector";
import { CompanyAdditionalPage } from "./additional-page";
import { CompanyAuthorizationPage } from "./authorization-page";

class TestResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

(
  globalThis as typeof globalThis & {
    ResizeObserver: typeof TestResizeObserver;
  }
).ResizeObserver = TestResizeObserver;
afterEach(() => cleanup());

function wrap(node: React.ReactNode) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      {node}
    </QueryClientProvider>,
  );
}

function withEmployeeLifecycle<
  T extends {
    users(input: { page: number; perPage: number }): Promise<{
      list: Array<{
        id: number;
        name: string;
        phone: string;
        status: number;
        version: number;
      }>;
      page: { total: number; totalPage: number };
    }>;
  },
>(api: T) {
  return {
    ...api,
    employees: async (input: { page: number; perPage: number }) => {
      const result = await api.users(input);
      return {
        ...result,
        list: result.list.map((user) => ({
          id: user.id,
          wxUserId: `wx-${user.id}`,
          name: user.name,
          mobile: user.phone,
          status: 1,
          account: {
            userId: user.id,
            loginIdentifier: user.phone,
            status: user.status,
            mustRotatePassword: false,
            authVersion: user.version,
          },
        })),
      };
    },
    provisionEmployeeAccount: vi.fn(),
    updateEmployeeAccountStatus: vi.fn(),
    resetEmployeePassword: vi.fn(),
  };
}

describe("access management pages", () => {
  it("groups and searches grantable permissions without exposing protected pages", () => {
    function SelectorHarness() {
      const [value, setValue] = React.useState<
        { code: string; scope: string }[]
      >([]);
      return (
        <AccessPermissionSelector
          ariaLabel="测试权限"
          catalog={[
            {
              id: 1,
              code: "dashboard.report.overview",
              path: "/reports/overview",
              name: "数据概览",
              groupCode: "data-reports",
              sort: 1,
              scopeRequired: true,
              superadminOnly: false,
            },
            {
              id: 2,
              code: "dashboard.conversation.all",
              path: "/chat/v2-all",
              name: "全局消息",
              groupCode: "conversation",
              sort: 2,
              scopeRequired: true,
              superadminOnly: false,
            },
            {
              id: 3,
              code: "dashboard.company.role",
              path: "/company/role",
              name: "角色管理",
              groupCode: "company-settings",
              sort: 3,
              scopeRequired: false,
              superadminOnly: true,
            },
          ]}
          value={value}
          onChange={setValue}
        />
      );
    }

    wrap(<SelectorHarness />);
    expect(screen.getByText("数据报表")).toBeTruthy();
    expect(screen.getByText("会话管理")).toBeTruthy();
    expect(screen.queryByLabelText("选择 角色管理")).toBeNull();
    fireEvent.change(screen.getByRole("searchbox", { name: "测试权限搜索" }), {
      target: { value: "conversation.all" },
    });
    expect(screen.queryByLabelText("选择 数据概览")).toBeNull();
    expect(screen.getByLabelText("选择 全局消息")).toBeTruthy();
  });

  it("selects multiple role permissions in one action and applies one scope to the selection", () => {
    function SelectorHarness() {
      const [value, setValue] = React.useState<
        { code: string; scope: string }[]
      >([]);
      return (
        <AccessPermissionSelector
          ariaLabel="角色页面权限"
          catalog={[
            {
              id: 1,
              code: "dashboard.report.overview",
              path: "/reports/overview",
              name: "数据概览",
              groupCode: "data-reports",
              sort: 1,
              scopeRequired: true,
              superadminOnly: false,
            },
            {
              id: 2,
              code: "dashboard.report.customer",
              path: "/reports/customer",
              name: "客户分析",
              groupCode: "data-reports",
              sort: 2,
              scopeRequired: true,
              superadminOnly: false,
            },
          ]}
          value={value}
          onChange={setValue}
        />
      );
    }

    wrap(<SelectorHarness />);
    fireEvent.click(
      screen.getByRole("button", { name: "全选 数据报表 组权限" }),
    );
    expect(screen.getByText("已选 2 / 2")).toBeTruthy();
    expect(screen.getByLabelText<HTMLInputElement>("选择 数据概览").checked).toBe(true);
    expect(screen.getByLabelText<HTMLInputElement>("选择 客户分析").checked).toBe(true);
    fireEvent.change(
      screen.getByRole("combobox", { name: "数据报表 批量数据范围" }),
      { target: { value: "tenant" } },
    );
    expect(
      screen.getByRole<HTMLSelectElement>("combobox", {
        name: "数据概览 数据范围",
      }).value,
    ).toBe("tenant");
    expect(
      screen.getByRole<HTMLSelectElement>("combobox", {
        name: "客户分析 数据范围",
      }).value,
    ).toBe("tenant");
  });

  it("loads summary then detail and requires confirmation before expectedVersion payload", async () => {
    const replaceUser = vi.fn().mockResolvedValue({});
    const api = {
      users: vi.fn().mockResolvedValue({
        list: [
          {
            id: 7,
            name: "张三",
            phone: "13800000000",
            status: 1,
            isSuperAdmin: false,
            version: 3,
          },
        ],
        page: { total: 1, totalPage: 1 },
      }),
      user: vi.fn().mockResolvedValue({
        id: 7,
        name: "张三",
        phone: "",
        status: 1,
        isSuperAdmin: false,
        version: 4,
        roles: [{ id: 2, name: "销售", status: 1, version: 1 }],
        directPermissions: [],
        inheritedPermissions: [
          {
            code: "x",
            path: "/index",
            name: "概览",
            scope: "department",
            sources: [
              { type: "role", id: 2, name: "销售", scope: "department" },
            ],
          },
        ],
        effectivePermissions: [],
      }),
      replaceUser,
      roles: vi.fn().mockResolvedValue({
        list: [
          { id: 2, name: "销售", status: 1, version: 1 },
          { id: 3, name: "停用", status: 2, version: 1 },
        ],
        page: { total: 2, totalPage: 1 },
      }),
      catalog: vi.fn().mockResolvedValue([
        {
          code: "p",
          name: "概览",
          groupCode: "data-reports",
          superadminOnly: false,
        },
        {
          code: "protected",
          name: "受保护",
          groupCode: "company-settings",
          superadminOnly: true,
        },
      ]),
    };
    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 390,
    });
    wrap(<AccessStaffPage api={withEmployeeLifecycle(api)} />);
    fireEvent.click(await screen.findByRole("button", { name: "权限设置" }));
    await waitFor(() => expect(api.user).toHaveBeenCalledWith(7));
    expect(await screen.findByText("数据报表")).toBeTruthy();
    expect(screen.getByText("已选 1 个角色 · 0 项直接权限")).toBeTruthy();
    expect(screen.queryByText(/继承权限/)).toBeNull();
    expect(screen.queryByText(/销售\/department/)).toBeNull();
    expect(screen.queryByLabelText("受保护")).toBeNull();
    expect(
      screen
        .getByRole("checkbox", { name: "停用（停用）" })
        .hasAttribute("disabled"),
    ).toBe(true);
    fireEvent.click(screen.getByRole("checkbox", { name: "选择 概览" }));
    fireEvent.change(
      screen
        .getAllByRole("combobox")
        .find((element) =>
          element.getAttribute("aria-label")?.includes("数据范围"),
        )!,
      { target: { value: "tenant" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "保存权限" }));
    expect(replaceUser).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole("button", { name: "确认" }));
    await waitFor(() =>
      expect(replaceUser).toHaveBeenCalledWith(
        7,
        expect.objectContaining({
          expectedVersion: 4,
          directPermissions: [{ code: "p", scope: "tenant" }],
        }),
      ),
    );
    expect(document.querySelector(".dashboard-table-scroll")).not.toBeNull();
    expect(window.innerWidth).toBe(390);
    expect(document.body.scrollWidth).toBeLessThanOrEqual(390);
  });

  it("shows role change summary, protected catalog filtering, system-role actions and 409 preservation", async () => {
    const updateRole = vi
      .fn()
      .mockRejectedValue(Object.assign(new Error("conflict"), { status: 409 }));
    const api = {
      roles: vi.fn().mockResolvedValue({
        list: [
          {
            id: 1,
            name: "系统",
            remark: "",
            status: 1,
            isSystem: true,
            memberCount: 0,
            permissions: [],
            version: 1,
          },
          {
            id: 2,
            name: "销售",
            remark: "",
            status: 1,
            isSystem: false,
            memberCount: 0,
            permissions: [],
            version: 2,
          },
        ],
        page: { total: 2, totalPage: 1 },
      }),
      catalog: vi.fn().mockResolvedValue([
        {
          id: 1,
          code: "p",
          path: "/index",
          name: "概览",
          groupCode: "data-reports",
          superadminOnly: false,
        },
        {
          id: 2,
          code: "protected",
          path: "/secret",
          name: "受保护",
          groupCode: "company-settings",
          superadminOnly: true,
        },
      ]),
      createRole: vi.fn(),
      updateRole,
      updateRoleStatus: vi.fn(),
      deleteRole: vi.fn(),
    };
    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 390,
    });
    wrap(<AccessRolePage api={api} />);
    const systemRow = (await screen.findByText("系统角色（不可修改）")).closest(
      "tr",
    );
    expect(systemRow?.querySelectorAll("button")).toHaveLength(0);
    fireEvent.click(screen.getByRole("button", { name: "编辑" }));
    fireEvent.change(screen.getByLabelText("角色名称"), {
      target: { value: "销售新" },
    });
    expect(screen.getByText("数据报表")).toBeTruthy();
    fireEvent.click(screen.getByRole("checkbox", { name: "选择 概览" }));
    fireEvent.change(
      screen
        .getAllByRole("combobox")
        .find((element) =>
          element.getAttribute("aria-label")?.includes("数据范围"),
        )!,
      { target: { value: "tenant" } },
    );
    expect(screen.queryByText("受保护")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(updateRole).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole("button", { name: "确认" }));
    await waitFor(() =>
      expect(updateRole).toHaveBeenCalledWith(
        2,
        expect.objectContaining({
          expectedVersion: 2,
          permissions: [{ code: "p", scope: "tenant" }],
        }),
      ),
    );
    expect((await screen.findByRole("alert")).textContent).toContain("409");
    expect(document.querySelector(".dashboard-table-scroll")).not.toBeNull();
    expect(window.innerWidth).toBe(390);
    expect(document.body.scrollWidth).toBeLessThanOrEqual(390);
  });

  it("does not allow saving when detail fails and loads role options across pages", async () => {
    const replaceUser = vi.fn();
    const api = {
      users: vi.fn().mockResolvedValue({
        list: [
          {
            id: 7,
            name: "张三",
            phone: "13800000000",
            status: 1,
            isSuperAdmin: false,
            version: 3,
          },
        ],
        page: { total: 1, totalPage: 1 },
      }),
      user: vi.fn().mockRejectedValue(new Error("detail failed")),
      replaceUser,
      roles: vi.fn().mockImplementation(({ page }: { page: number }) =>
        Promise.resolve({
          list:
            page === 1
              ? [{ id: 2, name: "销售", status: 1, version: 1 }]
              : [{ id: 99, name: "第二页角色", status: 1, version: 1 }],
          page: { total: 2, totalPage: 2 },
        }),
      ),
      catalog: vi
        .fn()
        .mockResolvedValue([
          { code: "p", name: "概览", superadminOnly: false },
        ]),
    };
    wrap(<AccessStaffPage api={withEmployeeLifecycle(api)} />);
    fireEvent.click(await screen.findByRole("button", { name: "权限设置" }));
    expect((await screen.findByRole("alert")).textContent).toContain(
      "账号权限加载失败",
    );
    expect(
      screen.getByRole("button", { name: "保存权限" }).hasAttribute("disabled"),
    ).toBe(true);
    expect(replaceUser).not.toHaveBeenCalled();
  });

  it("provisions an unbound employee only after confirmation and reveals the temporary password once", async () => {
    const provisionEmployeeAccount = vi.fn().mockResolvedValue({
      employee: {
        id: 4,
        wxUserId: "zhangsan",
        name: "张三",
        mobile: "13800000004",
        status: 1,
        account: {
          userId: 7,
          loginIdentifier: "13800000004",
          status: 1,
          mustRotatePassword: true,
          authVersion: 1,
        },
      },
      temporaryPassword: "DemoPass2026",
    });
    const api = {
      employees: vi.fn().mockResolvedValue({
        list: [
          {
            id: 4,
            wxUserId: "zhangsan",
            name: "张三",
            mobile: "13800000004",
            status: 1,
            account: null,
          },
        ],
        page: { total: 1, totalPage: 1 },
      }),
      provisionEmployeeAccount,
      updateEmployeeAccountStatus: vi.fn(),
      resetEmployeePassword: vi.fn(),
      user: vi.fn(),
      replaceUser: vi.fn(),
      roles: vi.fn().mockResolvedValue({
        list: [{ id: 2, name: "销售", status: 1, version: 1 }],
        page: { total: 1, totalPage: 1 },
      }),
      catalog: vi.fn().mockResolvedValue([
        {
          id: 1,
          code: "dashboard.report.overview",
          path: "/reports/overview",
          name: "数据概览",
          groupCode: "data-reports",
          sort: 1,
          scopeRequired: true,
          superadminOnly: false,
        },
      ]),
    };
    wrap(<AccessStaffPage api={api} />);
    expect(await screen.findByRole("columnheader", { name: "同步手机号" })).toBeTruthy();
    expect(await screen.findByRole("cell", { name: "13800000004" })).toBeTruthy();
    fireEvent.click(await screen.findByRole("button", { name: "开通账号" }));
    expect(screen.getByDisplayValue("13800000004")).toBeTruthy();
    expect(screen.getByText("企微账号：zhangsan")).toBeTruthy();
    expect(screen.getByText("同步手机号：13800000004")).toBeTruthy();
    fireEvent.click(screen.getByLabelText("销售（启用）"));
    fireEvent.click(screen.getByLabelText("选择 数据概览"));
    fireEvent.change(
      screen.getByRole("combobox", { name: "数据概览 数据范围" }),
      { target: { value: "department" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "确认开通" }));
    expect(provisionEmployeeAccount).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole("button", { name: "确认" }));
    await waitFor(() =>
      expect(provisionEmployeeAccount).toHaveBeenCalledWith(4, {
        loginIdentifier: "13800000004",
        roleIds: [2],
        directPermissions: [
          { code: "dashboard.report.overview", scope: "department" },
        ],
      }),
    );
    expect(await screen.findByText("DemoPass2026")).toBeTruthy();
    expect(screen.getByText(/首次登录必须修改密码/)).toBeTruthy();
  });

  it("shows a prominent specific error above the provisioning form", async () => {
    const provisionEmployeeAccount = vi.fn().mockRejectedValue({ status: 500 });
    const api = {
      employees: vi.fn().mockResolvedValue({
        list: [{ id: 4, wxUserId: "zhangsan", name: "张三", mobile: "13800000004", status: 1, account: null }],
        page: { total: 1, totalPage: 1 },
      }),
      provisionEmployeeAccount,
      updateEmployeeAccountStatus: vi.fn(),
      resetEmployeePassword: vi.fn(),
      user: vi.fn(),
      replaceUser: vi.fn(),
      roles: vi.fn().mockResolvedValue({ list: [], page: { total: 0, totalPage: 1 } }),
      catalog: vi.fn().mockResolvedValue([]),
    };
    wrap(<AccessStaffPage api={api} />);
    fireEvent.click(await screen.findByRole("button", { name: "开通账号" }));
    fireEvent.click(screen.getByRole("button", { name: "确认开通" }));
    fireEvent.click(await screen.findByRole("button", { name: "确认" }));
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("账号创建服务暂时不可用，请稍后重试");
    expect(alert.classList.contains("phase35-notice-error")).toBe(true);
    expect(alert.nextElementSibling?.classList.contains("access-provision-basics")).toBe(true);
  });

  it("confirms employee disable and password reset operations", async () => {
    const employee = {
      id: 4,
      wxUserId: "zhangsan",
      name: "张三",
      mobile: "13800000004",
      status: 1,
      account: {
        userId: 7,
        loginIdentifier: "13800000004",
        status: 1,
        mustRotatePassword: false,
        authVersion: 3,
      },
    };
    const updateEmployeeAccountStatus = vi
      .fn()
      .mockResolvedValue({ employee });
    const resetEmployeePassword = vi.fn().mockResolvedValue({
      employee,
      temporaryPassword: "ResetPass2026",
    });
    const api = {
      employees: vi.fn().mockResolvedValue({
        list: [employee],
        page: { total: 1, totalPage: 1 },
      }),
      provisionEmployeeAccount: vi.fn(),
      updateEmployeeAccountStatus,
      resetEmployeePassword,
      user: vi.fn(),
      replaceUser: vi.fn(),
      roles: vi.fn().mockResolvedValue({
        list: [],
        page: { total: 0, totalPage: 1 },
      }),
      catalog: vi.fn().mockResolvedValue([]),
    };
    wrap(<AccessStaffPage api={api} />);
    fireEvent.click(await screen.findByRole("button", { name: "停用" }));
    expect(updateEmployeeAccountStatus).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole("button", { name: "确认" }));
    await waitFor(() =>
      expect(updateEmployeeAccountStatus).toHaveBeenCalledWith(4, 2),
    );
    fireEvent.click(screen.getByRole("button", { name: "重置密码" }));
    fireEvent.click(await screen.findByRole("button", { name: "确认" }));
    await waitFor(() => expect(resetEmployeePassword).toHaveBeenCalledWith(4));
    expect(await screen.findByText("ResetPass2026")).toBeTruthy();
  });

  it("loads second role page and includes an unassigned role in the confirmed payload", async () => {
    const replaceUser = vi.fn().mockResolvedValue({});
    const api = {
      users: vi.fn().mockResolvedValue({
        list: [
          {
            id: 7,
            name: "张三",
            phone: "13800000000",
            status: 1,
            isSuperAdmin: false,
            version: 3,
          },
        ],
        page: { total: 1, totalPage: 1 },
      }),
      user: vi.fn().mockResolvedValue({
        id: 7,
        name: "张三",
        phone: "",
        status: 1,
        isSuperAdmin: false,
        version: 4,
        roles: [],
        directPermissions: [],
        inheritedPermissions: [],
        effectivePermissions: [],
      }),
      replaceUser,
      roles: vi.fn().mockImplementation(({ page }: { page: number }) =>
        Promise.resolve({
          list:
            page === 1
              ? [{ id: 2, name: "销售", status: 1, version: 1 }]
              : [{ id: 99, name: "第二页角色", status: 1, version: 1 }],
          page: { total: 2, totalPage: 2 },
        }),
      ),
      catalog: vi.fn().mockResolvedValue([]),
    };
    wrap(<AccessStaffPage api={withEmployeeLifecycle(api)} />);
    fireEvent.click(await screen.findByRole("button", { name: "权限设置" }));
    await waitFor(() =>
      expect(api.roles).toHaveBeenCalledWith({ page: 2, perPage: 100 }),
    );
    expect(await screen.findByText(/第二页角色/)).toBeTruthy();
    expect(api.roles).toHaveBeenCalledWith({ page: 2, perPage: 100 });
    fireEvent.click(screen.getByLabelText("第二页角色（启用）"));
    fireEvent.click(screen.getByRole("button", { name: "保存权限" }));
    fireEvent.click(await screen.findByRole("button", { name: "确认" }));
    await waitFor(() =>
      expect(replaceUser).toHaveBeenCalledWith(
        7,
        expect.objectContaining({ roleIds: [99], expectedVersion: 4 }),
      ),
    );
  });

  it("paginates employee summaries through the access users contract", async () => {
    const users = vi.fn().mockImplementation(({ page }: { page: number }) =>
      Promise.resolve({
        list: [
          {
            id: page,
            name: `员工${page}`,
            phone: "",
            status: 1,
            isSuperAdmin: false,
            version: 1,
          },
        ],
        page: { total: 51, totalPage: 2 },
      }),
    );
    const api = {
      users,
      user: vi.fn(),
      replaceUser: vi.fn(),
      roles: vi
        .fn()
        .mockResolvedValue({ list: [], page: { total: 0, totalPage: 1 } }),
      catalog: vi.fn().mockResolvedValue([]),
    };
    wrap(<AccessStaffPage api={withEmployeeLifecycle(api)} />);
    expect(await screen.findByText(/员工1/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    await waitFor(() =>
      expect(users).toHaveBeenCalledWith({ page: 2, perPage: 50 }),
    );
  });

  it("renders real catalog and audit access branches with empty/error states and pagination scroll", async () => {
    const catalogApi = {
      catalog: vi.fn().mockResolvedValue([
        {
          code: "p",
          path: "/overview",
          name: "概览",
          groupCode: "core",
          scopeRequired: false,
          superadminOnly: false,
        },
      ]),
    };
    wrap(<CompanyAdditionalPage api={catalogApi} />);
    expect(await screen.findByText("概览")).toBeTruthy();
    expect(document.querySelector(".dashboard-table-scroll")).not.toBeNull();

    const audits = vi.fn().mockResolvedValue({
      list: [
        {
          id: 1,
          actorUserId: 2,
          actorName: "管理员",
          action: "grant",
          targetType: "user",
          targetId: "7",
          before: null,
          after: null,
          expectedVersion: 1,
          resultVersion: 2,
          requestId: "req-1",
          time: "2026-08-10T00:00:00Z",
        },
      ],
      page: { page: 1, perPage: 50, total: 51, totalPage: 2 },
    });
    wrap(<CompanyAuthorizationPage api={{ audits }} />);
    expect(await screen.findByText("管理员")).toBeTruthy();
    expect(document.querySelector(".dashboard-table-scroll")).not.toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    await waitFor(() =>
      expect(audits).toHaveBeenCalledWith({ page: 2, perPage: 50 }),
    );
  });

  it("shows catalog and audit empty and retryable error states", async () => {
    const catalog = vi.fn().mockResolvedValue([]);
    wrap(<CompanyAdditionalPage api={{ catalog }} />);
    expect(await screen.findByText("暂无权限目录")).toBeTruthy();
    cleanup();
    const retryCatalog = vi.fn().mockRejectedValue(new Error("catalog failed"));
    wrap(<CompanyAdditionalPage api={{ catalog: retryCatalog }} />);
    expect(
      await screen.findByRole("button", { name: "重新加载" }),
    ).toBeTruthy();

    cleanup();
    const audits = vi.fn().mockResolvedValue({
      list: [],
      page: { page: 1, perPage: 50, total: 0, totalPage: 1 },
    });
    wrap(<CompanyAuthorizationPage api={{ audits }} />);
    expect(await screen.findByText("暂无审计记录")).toBeTruthy();
    cleanup();
    const retryAudits = vi.fn().mockRejectedValue(new Error("audit failed"));
    wrap(<CompanyAuthorizationPage api={{ audits: retryAudits }} />);
    expect(
      await screen.findByRole("button", { name: "重新加载" }),
    ).toBeTruthy();
  });

  it("paginates roles through the access roles contract", async () => {
    const roles = vi.fn().mockImplementation(({ page }: { page: number }) =>
      Promise.resolve({
        list: [
          {
            id: page,
            name: `角色${page}`,
            remark: "",
            status: 1,
            isSystem: false,
            memberCount: 0,
            permissions: [],
            version: 1,
          },
        ],
        page: { total: 51, totalPage: 2 },
      }),
    );
    const api = {
      roles,
      catalog: vi.fn().mockResolvedValue([]),
      createRole: vi.fn(),
      updateRole: vi.fn(),
      updateRoleStatus: vi.fn(),
      deleteRole: vi.fn(),
    };
    wrap(<AccessRolePage api={api} />);
    expect(await screen.findByText("角色1")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    await waitFor(() =>
      expect(roles).toHaveBeenCalledWith({ page: 2, perPage: 50 }),
    );
  });
});
