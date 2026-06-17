import Fastify from 'fastify';
import { db, pool } from './db/index.js';

const app = Fastify({ logger: true });
const port = Number(process.env.PORT ?? 3001);

app.get('/health', async () => ({ status: 'ok' }));

app.get('/health/db', async () => {
  const result = await db.execute('SELECT now() as now');
  return { status: 'ok', now: result.rows[0]?.now };
});

async function start() {
  try {
    await app.listen({ port, host: '0.0.0.0' });
  } catch (error) {
    app.log.error(error);
    await pool.end();
    process.exit(1);
  }
}

start();
