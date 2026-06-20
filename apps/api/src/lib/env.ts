import { config } from 'dotenv';
import { existsSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

/**
 * Load `.env` from the project root by walking up from the current module.
 * This allows the same built/TSX entry to work regardless of cwd.
 */
export function loadEnvFromProjectRoot(): void {
  const moduleDir = dirname(fileURLToPath(import.meta.url));
  let envDir = moduleDir;
  while (envDir !== dirname(envDir)) {
    const envPath = resolve(envDir, '.env');
    if (existsSync(envPath)) {
      config({ path: envPath });
      break;
    }
    envDir = dirname(envDir);
  }
}
