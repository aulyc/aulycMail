import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import path from 'path'

// Contacts lives outside frontend/ at the repo root (../extensions/contacts/...).
// $contacts aliases that directory so App.svelte and contacts files can import
// components and stores cleanly. $wailsjs aliases the generated Wails bindings
// so deep contacts files don't need ../ chains.
//
// Contacts Svelte/TS files live outside frontend/, so Vite needs an explicit
// filesystem allow-list and a few dependency aliases back to frontend/node_modules.
const CONTACTS_DIR = path.resolve(__dirname, '../extensions/contacts')
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
          if (id.includes('/extensions/contacts/')) return 'contacts'
          if (id.includes('/node_modules/')) return 'vendor'
        },
      },
    },
  },
  server: {
    strictPort: true,
    fs: {
      // Vite blocks file reads outside its root by default. Contacts lives
      // one level above the frontend root, so allow that directory.
      allow: ['..', CONTACTS_DIR],
    },
  },
})
