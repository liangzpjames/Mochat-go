import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    clearMocks: true,
    environment: 'jsdom',
    fileParallelism: false,
    restoreMocks: true,
  },
});
