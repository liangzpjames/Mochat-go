import type { SidebarRequest } from '../app/sidebar-router';

export type WeComMessage =
  | { type: 'text'; content: string }
  | { type: 'news'; link: string; title: string; description: string; imageUrl: string }
  | { type: 'image' | 'video' | 'file'; mediaId: string };

export type WeComSDK = {
  agentConfig(input: {
    corpid: string;
    agentid: string;
    timestamp: number;
    nonceStr: string;
    signature: string;
    jsApiList: string[];
    success: () => void;
    fail: (error: Record<string, unknown>) => void;
  }): void;
  invoke(name: string, payload: unknown, callback: (result: Record<string, unknown>) => void): void;
};

export class WeComBridgeError extends Error {
  readonly kind: 'unavailable' | 'configuration' | 'invoke';

  constructor(kind: WeComBridgeError['kind'], message: string) {
    super(message);
    this.name = 'WeComBridgeError';
    this.kind = kind;
  }
}

export type WeComBridge = {
  available(): boolean;
  sendChatMessage(message: WeComMessage): Promise<void>;
  navigateToAddCustomer(): Promise<void>;
};

export function createWeComBridge(_input: {
  request: SidebarRequest;
  agentId: () => string | null;
  href: () => string;
  sdk: () => WeComSDK | undefined;
}): WeComBridge {
  const input = _input;
  let configured: Promise<WeComSDK> | null = null;

  const ensureConfigured = (): Promise<WeComSDK> => {
    const sdk = input.sdk();
    if (sdk === undefined) {
      return Promise.reject(new WeComBridgeError(
        'unavailable',
        '企业微信能力不可用，请在企业微信客户端中打开。',
      ));
    }
    if (configured !== null) return configured;
    configured = (async () => {
      const agentId = input.agentId();
      if (agentId === null || !/^[1-9]\d*$/.test(agentId)) {
        throw new WeComBridgeError('configuration', '缺少有效的企业应用 ID。');
      }
      const url = new URL(input.href());
      url.hash = '';
      const query = new URLSearchParams({ agentId, uriPath: url.toString() });
      const raw = await input.request<unknown>(`/agent/jssdkConfig?${query.toString()}`, {
        method: 'GET',
      });
      if (typeof raw !== 'object' || raw === null) {
        throw new WeComBridgeError('configuration', '企业微信签名配置无效。');
      }
      const config = raw as Record<string, unknown>;
      if (
        typeof config.corpid !== 'string'
        || typeof config.agentid !== 'string'
        || typeof config.timestamp !== 'number'
        || typeof config.nonceStr !== 'string'
        || typeof config.signature !== 'string'
      ) {
        throw new WeComBridgeError('configuration', '企业微信签名配置无效。');
      }
      await new Promise<void>((resolve, reject) => {
        sdk.agentConfig({
          corpid: config.corpid as string,
          agentid: config.agentid as string,
          timestamp: config.timestamp as number,
          nonceStr: config.nonceStr as string,
          signature: config.signature as string,
          jsApiList: ['sendChatMessage', 'navigateToAddCustomer'],
          success: resolve,
          fail: () => reject(new WeComBridgeError(
            'configuration',
            '企业微信签名校验失败，请重试。',
          )),
        });
      });
      return sdk;
    })().catch((error: unknown) => {
      configured = null;
      throw error;
    });
    return configured;
  };

  const invoke = async (name: string, payload: unknown): Promise<void> => {
    const sdk = await ensureConfigured();
    await new Promise<void>((resolve, reject) => {
      sdk.invoke(name, payload, (result) => {
        const message = typeof result.err_msg === 'string'
          ? result.err_msg
          : typeof result.errMsg === 'string' ? result.errMsg : '';
        if (message === `${name}:ok`) {
          resolve();
          return;
        }
        reject(new WeComBridgeError('invoke', '企业微信操作未完成，请重试。'));
      });
    });
  };

  const messagePayload = (message: WeComMessage): unknown => {
    if (message.type === 'text') {
      return { msgtype: 'text', text: { content: message.content } };
    }
    if (message.type === 'news') {
      return {
        msgtype: 'news',
        news: {
          link: message.link,
          title: message.title,
          desc: message.description,
          imgUrl: message.imageUrl,
        },
      };
    }
    return { msgtype: message.type, [message.type]: { mediaid: message.mediaId } };
  };

  return {
    available: () => input.sdk() !== undefined,
    sendChatMessage: (message) => invoke('sendChatMessage', messagePayload(message)),
    navigateToAddCustomer: () => invoke('navigateToAddCustomer', {}),
  };
}
