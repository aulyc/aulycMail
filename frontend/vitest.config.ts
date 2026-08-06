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
          statements: 92.5,
          branches: 80.5,
          functions: 94.5,
          lines: 94.5,
        },
      },
    },
  }),
)
