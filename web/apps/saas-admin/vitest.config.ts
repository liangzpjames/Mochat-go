import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    include: ['vite.config.test.ts'],
    exclude: ['node_modules/**', 'dist/**'],
  },
})
