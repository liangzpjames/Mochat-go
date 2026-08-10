import * as React from "react";
import type { AccessCatalogItem } from "../access/access-api";

export type PermissionSelection = {
  code: string;
  scope: string;
};

type Props = {
  ariaLabel: string;
  catalog: AccessCatalogItem[];
  value: PermissionSelection[];
  onChange: (value: PermissionSelection[]) => void;
};

const groupNames: Record<string, string> = {
  overview: "核心页面",
  conversation: "会话管理",
  "risk-warning": "风险预警",
  "ai-insight": "AI 洞察",
  "marketing-tools": "营销工具",
  scrm: "客户管理",
  "data-reports": "数据报表",
  "ai-settings": "AI 设置",
  "company-settings": "企业设置",
};

function groupCode(item: AccessCatalogItem) {
  return item.groupCode || "overview";
}

export function AccessPermissionSelector({
  ariaLabel,
  catalog,
  value,
  onChange,
}: Props) {
  const [keyword, setKeyword] = React.useState("");
  const normalizedKeyword = keyword.trim().toLocaleLowerCase();
  const grantable = catalog
    .filter((item) => !item.superadminOnly)
    .filter(
      (item) =>
        normalizedKeyword.length === 0 ||
        item.name.toLocaleLowerCase().includes(normalizedKeyword) ||
        item.code.toLocaleLowerCase().includes(normalizedKeyword),
    )
    .sort((left, right) => (left.sort ?? 0) - (right.sort ?? 0));
  const groups = new Map<string, AccessCatalogItem[]>();
  for (const item of grantable) {
    const code = groupCode(item);
    groups.set(code, [...(groups.get(code) ?? []), item]);
  }

  return (
    <fieldset className="access-permission-editor" aria-label={ariaLabel}>
      <legend>{ariaLabel}</legend>
      <div className="access-permission-toolbar">
        <label>
          <span>搜索页面权限</span>
          <input
            aria-label={`${ariaLabel}搜索`}
            type="search"
            value={keyword}
            placeholder="输入页面名称或权限 code"
            onChange={(event) => setKeyword(event.target.value)}
          />
        </label>
        <span className="access-permission-total">
          已选 {value.length} / {catalog.filter((item) => !item.superadminOnly).length}
        </span>
      </div>
      <div className="access-permission-groups">
        {[...groups].map(([code, items]) => {
          const selectedCount = items.filter((item) =>
            value.some((permission) => permission.code === item.code),
          ).length;
          return (
            <section className="access-permission-group" key={code}>
              <header>
                <h3>{groupNames[code] ?? code}</h3>
                <span>
                  {selectedCount}/{items.length}
                </span>
              </header>
              <div className="access-permission-list">
                {items.map((item) => {
                  const selected = value.find(
                    (permission) => permission.code === item.code,
                  );
                  return (
                    <div className="access-permission-row" key={item.code}>
                      <label className="access-permission-check">
                        <input
                          aria-label={`选择 ${item.name}`}
                          type="checkbox"
                          checked={selected !== undefined}
                          onChange={() =>
                            onChange(
                              selected
                                ? value.filter(
                                    (permission) =>
                                      permission.code !== item.code,
                                  )
                                : [
                                    ...value,
                                    { code: item.code, scope: "self" },
                                  ],
                            )
                          }
                        />
                        <span>
                          <strong>{item.name}</strong>
                          <small>{item.code}</small>
                        </span>
                      </label>
                      <label className="access-permission-scope">
                        <span>数据范围</span>
                        <select
                          aria-label={`${item.name} 数据范围`}
                          value={selected?.scope ?? "self"}
                          disabled={!selected}
                          onChange={(event) =>
                            onChange(
                              value.map((permission) =>
                                permission.code === item.code
                                  ? {
                                      ...permission,
                                      scope: event.target.value,
                                    }
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
                    </div>
                  );
                })}
              </div>
            </section>
          );
        })}
        {grantable.length === 0 && (
          <p className="access-permission-empty">没有匹配的页面权限</p>
        )}
      </div>
    </fieldset>
  );
}
