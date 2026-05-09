import { createServer } from 'vite'
import vue from '@vitejs/plugin-vue'

const server = await createServer({
  configFile: false,
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 5173
  }
})

await server.listen()
console.log('fallback-vite-ready on http://127.0.0.1:5173')

process.stdin.resume()
