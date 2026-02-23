import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        search: './app.ts',
        site: './site.ts'
      },
      output: {
        entryFileNames: '[name].js',
        assetFileNames: '[name]-[hash][extname]'
      }
    }
  }
});
