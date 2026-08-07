export type AudioObject = {
  id: number;
  originalName: string;
  contentType: string;
  sizeBytes: number;
  durationSeconds: number;
  createdAt: string;
  playUrl: string;
};

export type AudioListResult = {
  list: AudioObject[];
  total: number;
  page: number;
  perPage: number;
};

type Client = {
  request<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T>;
};

export function createFileAudioApi(client: Client) {
  return {
    list(corpId: number, page: number, perPage: number, keyword = ''): Promise<AudioListResult> {
      const params = new URLSearchParams({
        corpId: String(corpId),
        page: String(page),
        perPage: String(perPage),
      });
      if (keyword.trim() !== '') {
        params.set('keyword', keyword.trim());
      }
      return client.request<AudioListResult>(`/chat/media?${params.toString()}`);
    },
    upload(corpId: number, file: File): Promise<{ id: number; playUrl: string }> {
      const form = new FormData();
      form.append('file', file);
      return client.request<{ id: number; playUrl: string }>(`/chat/media?corpId=${corpId}`, {
        method: 'POST',
        body: form,
      });
    },
    remove(corpId: number, id: number): Promise<{ id: number }> {
      return client.request<{ id: number }>(`/chat/media/${id}?corpId=${corpId}`, {
        method: 'DELETE',
      });
    },
  };
}

export type FileAudioApi = ReturnType<typeof createFileAudioApi>;
