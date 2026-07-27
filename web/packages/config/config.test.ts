import { describe, expect, it } from 'vitest';
import eslintConfig from '@mochat/config/eslint';
import vitestConfig from '@mochat/config/vitest';

describe('@mochat/config exports', () => {
  it('provides a usable Vitest base configuration', () => {
    expect(vitestConfig.test).toMatchObject({
      clearMocks: true,
      mockReset: true,
      restoreMocks: true,
    });
  });

  it('provides a TypeScript-aware ESLint flat configuration', () => {
    expect(eslintConfig.some((config) => config.languageOptions?.parserOptions?.projectService === true)).toBe(true);
    expect(eslintConfig.some((config) => config.rules?.['@typescript-eslint/no-unused-vars'])).toBe(true);
  });
});
