export type PasswordUpdateInput = {
  oldPassword: string;
  newPassword: string;
  againNewPassword: string;
};

type ApiClient = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};

export function createPasswordApi(client: ApiClient) {
  return {
    async update(input: PasswordUpdateInput): Promise<void> {
      await client.request('/user/passwordUpdate', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(input),
      });
    },
  };
}
