import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/__tests__/setup.js'],
    include: ['src/**/*.{test,spec}.js'],
    // Forks paralelos en Windows + jsdom dan flakes intermitentes (asserts de
    // dialogs fallan solo bajo carga). Serial = determinista; ~35s para 5 files.
    fileParallelism: false,
  },
});
