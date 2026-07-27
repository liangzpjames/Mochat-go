import { describe, expect, it } from 'vitest';
import eslintConfig from '@mochat/config/eslint';
import vitestConfig from '@mochat/config/vitest';
import packageManifest from './package.json';

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

  it('pins tooling compatible with the complete Node engine range', () => {
    expect(packageManifest.dependencies).toMatchObject({
      '@eslint/js': '9.39.5',
      eslint: '9.39.5',
      typescript: '5.9.3',
      'typescript-eslint': '8.65.0',
      vite: '6.4.3',
      vitest: '2.1.9',
    });
  });
});
