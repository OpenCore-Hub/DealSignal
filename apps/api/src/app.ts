import Fastify from 'fastify';
import { db } from './db/index.js';
import { registerAuthRoutes, type AuthRoutesOptions } from './modules/auth/routes.js';
import { registerStorageRoutes, type StorageRoutesOptions } from './modules/storage/routes.js';

export type CreateAppOptions = AuthRoutesOptions & StorageRoutesOptions;

export async function createApp(options: CreateAppOptions = {}) {
  const app = Fastify({ logger: true });

  app.get('/health', async () => ({ status: 'ok' }));

  app.get('/health/db', async () => {
    const result = await db.execute('SELECT now() as now');
    return { status: 'ok', now: result.rows[0]?.now };
  });

  await registerAuthRoutes(app, options);
  await registerStorageRoutes(app, options);

  return app;
}
