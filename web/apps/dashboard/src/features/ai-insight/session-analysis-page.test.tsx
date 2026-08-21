import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { SessionAnalysisPage } from './session-analysis-page';

const api = { sessionRecords: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 0, items: [] }), sessionStatus: vi.fn().mockResolvedValue({ provider: { state: 'ready' } }), sessionDetail: vi.fn(), sessionExportUrl: vi.fn().mockReturnValue('/export') } as never;
describe('会话分析工作台', () => { it('使用紧凑查询和空状态，不显示能力说明大卡', async () => { render(<SessionAnalysisPage api={api} />); expect(await screen.findByRole('heading', { name: '会话分析' })).toBeTruthy(); expect(screen.getByRole('button', { name: '查询' })).toBeTruthy(); expect(screen.getByText('当前筛选暂无分析结果')).toBeTruthy(); expect(screen.queryByText('AI 能力未接入')).toBeNull(); }); });
