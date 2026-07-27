import { describe, expect, it } from 'vitest'

import config from './vite.config'

describe('Vite configuration', () => {
  it('serves the SaaS Admin public base and writes an isolated dist directory', () => {
    expect(config).toMatchObject({
      base: '/saas-admin/',
      build: {
        outDir: 'dist',
        emptyOutDir: true,
      },
    })
  })
})
