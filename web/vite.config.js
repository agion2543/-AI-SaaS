import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    historyApiFallback: {
      rewrites: [
        { from: /^\/merchant\/.*$/, to: '/index.html' },
        { from: /^\/admin\/.*$/, to: '/index.html' },
        { from: /^\/portal\/.*$/, to: '/index.html' },
        { from: /^\/customer\/.*$/, to: '/index.html' },
        { from: /^\/payment\/.*$/, to: '/index.html' },
        { from: /^\/auth\/.*$/, to: '/index.html' }
      ]
    },
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/uploads': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  build: {
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'index.html'),
        mobile: resolve(__dirname, 'mobile.html')
      }
    }
  }
})
