export type AudioObject = {
  id: number;
  originalName: string;
  source: string;
  messageId: string;
  senderName: string;
  receiverName: string;
  contentType: string;
  sizeBytes: number;
  durationSeconds: number;
  createdAt: string;
  syncedAt?: string;
  playUrl: string;
};

export type AudioListResult = {
  list: AudioObject[];
  total: number;
  page: number;
  perPage: number;
};

export type AudioListFilter = {
  sender: string;
  receiver: string;
  from: string;
  to: string;
};

type Client = {
  request<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T>;
  download?: (input: RequestInfo | URL, init?: RequestInit) => Promise<{ blob: Blob; filename: string }>;
};

export type FileAudioApi = {
  list(page: number, perPage: number, filter: AudioListFilter): Promise<AudioListResult>;
  content?: (id: number) => Promise<Blob>;
};

export function createFileAudioApi(client: Client): FileAudioApi {
  return {
    list(page: number, perPage: number, filter: AudioListFilter): Promise<AudioListResult> {
      const params = new URLSearchParams({
        page: String(page),
        perPage: String(perPage),
      });
      if (filter.sender.trim() !== '') params.set('sender', filter.sender.trim());
      if (filter.receiver.trim() !== '') params.set('receiver', filter.receiver.trim());
      if (filter.from) params.set('from', filter.from);
      if (filter.to) params.set('to', filter.to);
      return client.request<AudioListResult>(`/chat/media?${params.toString()}`);
    },
    content(id: number): Promise<Blob> {
      if (client.download === undefined) {
        return Promise.reject(new Error('录音播放请求未配置'));
      }
      return client.download(`/chat/media/${id}/content`).then((result) => result.blob);
    },
  };
}
