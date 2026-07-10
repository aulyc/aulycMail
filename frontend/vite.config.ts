import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import path from 'path'

const CONTACTS_DIR = path.resolve(__dirname, './src/lib/contacts')
const WAILSJS_DIR = path.resolve(__dirname, './wailsjs')
const NODE_MODULES_DIR = path.resolve(__dirname, './node_modules')

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelte()],
  resolve: {
    alias: {
      '$lib': path.resolve('./src/lib'),
      '$': path.resolve('./src'),
      '$contacts': CONTACTS_DIR,
      '$wailsjs': WAILSJS_DIR,
      '@iconify/svelte': path.resolve(NODE_MODULES_DIR, '@iconify/svelte'),
      'svelte-i18n':     path.resolve(NODE_MODULES_DIR, 'svelte-i18n'),
    },
  },
  optimizeDeps: {
    include: ['@iconify-json/mdi', '@iconify-json/lucide', '@iconify-json/heroicons', '@iconify-json/logos', '@iconify-json/simple-icons'],
  },
  build: {
    target: 'esnext',
    minify: 'esbuild',
    sourcemap: false,
    // Desktop app bundle: after manual vendor splitting, the remaining vendor
    // chunk is expected to be under this cap and no longer masks app-code size.
    chunkSizeWarningLimit: 700,
    rollupOptions: {
      input: {
        main: path.resolve(__dirname, 'index.html'),
      },
      output: {
        manualChunks(id) {
          if (id.includes('/wailsjs/')) return 'wails'
          if (id.includes('/src/lib/contacts/')) return 'contacts'
          if (id.includes('/node_modules/')) return 'vendor'
        },
      },
    },
  },
  server: {
    strictPort: true,
  },
})
