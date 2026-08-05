import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { mergeConfig } from 'vite'
import { defineConfig } from 'vitest/config'

import viteConfig from './vite.config'

const frontendRoot = path.dirname(fileURLToPath(import.meta.url))

export default mergeConfig(
  viteConfig,
  defineConfig({
    resolve: {
      conditions: ['browser'],
    },
    test: {
      environment: 'node',
      include: ['scripts/*.test.mjs'],
      coverage: {
        provider: 'v8',
        include: ['src/**/*.ts', 'src/**/*.svelte'],
        exclude: ['src/**/*.d.ts'],
        reporter: ['text', 'json-summary'],
        reportsDirectory: path.resolve(frontendRoot, '../.cache/coverage/frontend'),
        thresholds: {
          statements: 91.5,
          branches: 78.75,
          functions: 94,
          lines: 93.25,
        },
      },
    },
  }),
)
