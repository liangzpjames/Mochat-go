type ApiClient = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};

export function createBusinessWorkbenchApi(client: ApiClient) {
  return {
    read(endpoint: string, values: Record<string, string | number>): Promise<unknown> {
      const query = new URLSearchParams(
        Object.entries(values).map(([key, value]) => [key, String(value)]),
      );
      return client.request(`${endpoint}?${query.toString()}`);
    },
    write(endpoint: string, values: Record<string, unknown>, method: 'POST' | 'PUT' | 'DELETE' = 'POST'): Promise<unknown> {
      return client.request(endpoint, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(values),
      });
    },
  };
}
