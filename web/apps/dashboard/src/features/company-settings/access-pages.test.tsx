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

describe("access management pages", () => {
  it("loads summary then detail and requires confirmation before expectedVersion payload", async () => {
    const replaceUser = vi.fn().mockResolvedValue({});
    const api = {
      users: vi
        .fn()
        .mockResolvedValue({
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
      user: vi
        .fn()
        .mockResolvedValue({
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
      }),
      catalog: vi.fn().mockResolvedValue([
        { code: "p", name: "概览", superadminOnly: false },
        { code: "protected", name: "受保护", superadminOnly: true },
      ]),
    };
    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 390,
    });
    wrap(<AccessStaffPage api={api} />);
    fireEvent.click(await screen.findByRole("button", { name: "编辑权限" }));
    await waitFor(() => expect(api.user).toHaveBeenCalledWith(7));
    expect(await screen.findByText(/继承权限/)).toBeTruthy();
    expect(screen.queryByLabelText("受保护")).toBeNull();
    expect(
      screen
        .getByRole("checkbox", { name: "停用（停用）" })
        .hasAttribute("disabled"),
    ).toBe(true);
    fireEvent.click(screen.getAllByRole("checkbox")[2]!);
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
          superadminOnly: false,
        },
        {
          id: 2,
          code: "protected",
          path: "/secret",
          name: "受保护",
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
    fireEvent.click(screen.getAllByRole("checkbox")[0]!);
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
});
