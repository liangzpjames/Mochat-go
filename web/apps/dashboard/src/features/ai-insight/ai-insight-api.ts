export type AiInsightPageResult = {
  page: string; title: string; capability: 'ready' | 'limited' | 'unavailable';
  provider: string; limitations: string[]; data: unknown[];
};

type Client = { request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown> };

export function createAiInsightApi(client: Client) {
  return {
    read(page: string, corpId: number): Promise<AiInsightPageResult> {
      return client.request(`/ai-insight/${page}?corpId=${corpId}`) as Promise<AiInsightPageResult>;
    },
  };
}

export type AiInsightApi = ReturnType<typeof createAiInsightApi>;
