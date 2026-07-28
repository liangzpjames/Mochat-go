export type CorpOption = {
  id: string;
  name: string;
  authorized: boolean;
};

type ApiClient = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};

type CorpResponse = {
  corpId: number;
  corpName: string;
};

export async function loadCorps(client: ApiClient): Promise<CorpOption[]> {
  const corps = await client.request('/corp/select') as CorpResponse[];
  return corps.map((corp) => ({
    id: String(corp.corpId),
    name: corp.corpName,
    authorized: true,
  }));
}

export async function bindCorp(client: ApiClient, corpId: string): Promise<void> {
  await client.request('/corp/bind', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ corpId: Number(corpId) }),
  });
}
