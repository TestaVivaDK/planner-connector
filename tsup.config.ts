import { defineConfig } from 'tsup';

export default defineConfig({
  entry: ['src/**/*.ts', 'src/endpoints.json'],
  format: ['esm'],
  target: 'es2020',
  outDir: 'dist',
  clean: true,
  bundle: false,
  splitting: false,
  sourcemap: false,
  dts: false,
  onSuccess: 'chmod +x dist/index.js',
  loader: {
    '.json': 'copy',
  },
  external: [
    '@azure/msal-node',
    '@modelcontextprotocol/sdk',
    'commander',
    'dotenv',
    'express',
    'keytar',
    'winston',
    'zod',
  ],
});
