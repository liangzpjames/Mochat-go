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
    async write(endpoint: string, values: Record<string, unknown>): Promise<void> {
      await client.request(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(values),
      });
    },
  };
}
