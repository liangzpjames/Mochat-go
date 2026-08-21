import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { SmartAnalysisPage } from './smart-analysis-page';

const api = { smartRecords: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 0, items: [] }), smartStatus: vi.fn().mockResolvedValue({ provider: { state: 'ready' } }), smartRules: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 0, items: [] }), smartDetail: vi.fn(), createRule: vi.fn(), updateRule: vi.fn(), setRuleStatus: vi.fn(), deleteRule: vi.fn() } as never;
describe('智能分析工作台', () => { it('提供分析结果与分析规则两个标签', async () => { render(<SmartAnalysisPage api={api} />); expect(screen.getByRole('tab', { name: '分析结果' })).toBeTruthy(); expect(screen.getByRole('tab', { name: '分析规则' })).toBeTruthy(); expect(await screen.findByText('当前暂无匹配的规则分析结果')).toBeTruthy(); }); });
