export type DepartmentListInput = {
  name: string;
  parentName: string;
  page: number;
  perPage: number;
};

export type DepartmentItem = {
  departmentId: number;
  departmentPath: string;
  name: string;
  level: string;
  children?: DepartmentItem[];
};

export type PageMeta = {
  page: number;
  perPage: number;
  total: number;
  totalPage: number;
};

export type DepartmentListResult = { list: DepartmentItem[]; page: PageMeta };
export type DepartmentMember = {
  employeeId: number;
  employeeName: string;
  phone: string;
  roleName: string;
};
export type DepartmentMemberResult = { list: DepartmentMember[]; page: PageMeta };
export type DepartmentMemberInput = {
  departmentId: number;
  page: number;
  perPage: number;
};

type ApiClient = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};

export function createDepartmentApi(client: ApiClient) {
  return {
    list(input: DepartmentListInput): Promise<DepartmentListResult> {
      const query = new URLSearchParams({
        name: input.name,
        parentName: input.parentName,
        page: String(input.page),
        perPage: String(input.perPage),
      });
      return client.request(`/workDepartment/pageIndex?${query.toString()}`) as Promise<DepartmentListResult>;
    },
    members(input: DepartmentMemberInput): Promise<DepartmentMemberResult> {
      const query = new URLSearchParams({
        departmentId: String(input.departmentId),
        page: String(input.page),
        perPage: String(input.perPage),
      });
      return client.request(`/workDepartment/showEmployee?${query.toString()}`) as Promise<DepartmentMemberResult>;
    },
  };
}
