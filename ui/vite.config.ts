import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    vue(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['icon.svg', 'apple-touch-icon.png'],
      manifest: {
        name: 'SSH Tunnel Service',
        short_name: 'SSH Tunnels',
        description: 'Manage SSH port-forwarding tunnels',
        theme_color: '#2563eb',
        background_color: '#f1f5f9',
        display: 'standalone',
        start_url: '/',
        // A self-contained SVG plus raster fallbacks. The maskable PNG keeps all
        // artwork inside the safe zone so the launcher mask never clips it.
        icons: [
          { src: '/icon.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any' },
          { src: '/pwa-192x192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
          { src: '/pwa-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'any' },
          { src: '/pwa-maskable-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' }
        ]
      },
      workbox: {
        // index.html is excluded: it carries the server-injected auth token and must
        // never be served stale from the cache. Only immutable static assets are precached.
        globPatterns: ['**/*.{js,css,svg,png,woff2}'],
        navigateFallback: null,
        runtimeCaching: []
      }
    })
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:2222',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: '../internal/web/static',
    emptyOutDir: true
  }
})
