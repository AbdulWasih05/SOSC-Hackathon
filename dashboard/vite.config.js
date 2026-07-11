import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Minimal Vite config. The dashboard talks to the engine directly over
// ws://localhost:8080 and http://localhost:8080 (CORS enabled on the engine),
// so no dev proxy is needed.
export default defineConfig({
  plugins: [react()],
})
