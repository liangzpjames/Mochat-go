import { describe, expect, it, vi } from 'vitest';

import { createWeComBridge, WeComBridgeError, type WeComSDK } from './wecom-bridge';

function sdkFixture(invokeResult = { err_msg: 'sendChatMessage:ok' }) {
  const invoke = vi.fn((_name: string, _payload: unknown, callback: (result: Record<string, unknown>) => void) => callback(invokeResult));
  const agentConfig = vi.fn((config: Record<string, unknown>) => {
    const success = config.success as (() => void);
    success();
  });
  return { sdk: { invoke, agentConfig } satisfies WeComSDK, invoke, agentConfig };
}

const config = {
  corpid: 'wx-corp',
  agentid: '100001',
  timestamp: 123,
  nonceStr: 'nonce',
  signature: 'signature',
};

describe('WeCom bridge', () => {
  it('fails honestly outside the WeCom host and sends no API request', async () => {
    const request = vi.fn();
    const bridge = createWeComBridge({ request, agentId: () => '7', href: () => 'https://sidebar.test/contact', sdk: () => undefined });

    await expect(bridge.sendChatMessage({ type: 'text', content: '你好' }))
      .rejects.toBeInstanceOf(WeComBridgeError);
    expect(request).not.toHaveBeenCalled();
  });

  it('configures the agent once and invokes exact text and news payloads', async () => {
    const request = vi.fn().mockResolvedValue(config);
    const fixture = sdkFixture();
    const bridge = createWeComBridge({ request, agentId: () => '7', href: () => 'https://sidebar.test/contact#tab', sdk: () => fixture.sdk });

    await bridge.sendChatMessage({ type: 'text', content: '你好' });
    await bridge.sendChatMessage({ type: 'news', link: 'https://example.com/a', title: '标题', description: '说明', imageUrl: 'https://example.com/a.png' });

    expect(request).toHaveBeenCalledTimes(1);
    expect(request).toHaveBeenCalledWith('/agent/jssdkConfig?agentId=7&uriPath=https%3A%2F%2Fsidebar.test%2Fcontact', { method: 'GET' });
    expect(fixture.invoke).toHaveBeenNthCalledWith(1, 'sendChatMessage', { msgtype: 'text', text: { content: '你好' } }, expect.any(Function));
    expect(fixture.invoke).toHaveBeenNthCalledWith(2, 'sendChatMessage', { msgtype: 'news', news: { link: 'https://example.com/a', title: '标题', desc: '说明', imgUrl: 'https://example.com/a.png' } }, expect.any(Function));
  });

  it('refreshes media ids through the caller and sends persisted media payloads', async () => {
    const request = vi.fn().mockResolvedValue(config);
    const fixture = sdkFixture({ err_msg: 'sendChatMessage:ok' });
    const bridge = createWeComBridge({ request, agentId: () => '7', href: () => 'https://sidebar.test/medium', sdk: () => fixture.sdk });

    await bridge.sendChatMessage({ type: 'file', mediaId: 'media-7' });

    expect(fixture.invoke).toHaveBeenCalledWith('sendChatMessage', { msgtype: 'file', file: { mediaid: 'media-7' } }, expect.any(Function));
  });

  it('surfaces agentConfig and invoke failures without reporting success', async () => {
    const request = vi.fn().mockResolvedValue(config);
    const failingConfig: WeComSDK = {
      agentConfig: (input) => input.fail({ errMsg: 'agentConfig:fail invalid signature' }),
      invoke: vi.fn(),
    };
    const bridge = createWeComBridge({ request, agentId: () => '7', href: () => 'https://sidebar.test/medium', sdk: () => failingConfig });

    await expect(bridge.navigateToAddCustomer()).rejects.toMatchObject({ kind: 'configuration' });

    const invokeFailure = sdkFixture({ err_msg: 'navigateToAddCustomer:fail no permission' });
    const second = createWeComBridge({ request, agentId: () => '7', href: () => 'https://sidebar.test/contactBatchAdd', sdk: () => invokeFailure.sdk });
    await expect(second.navigateToAddCustomer()).rejects.toMatchObject({ kind: 'invoke' });
  });
});
