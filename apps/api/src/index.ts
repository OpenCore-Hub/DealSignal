import { pool } from './db/index.js';
import { createApp } from './app.js';

const port = Number(process.env.PORT ?? 3001);
const app = await createApp();

async function start() {
  try {
    await app.listen({ port, host: '0.0.0.0' });
  } catch (error) {
    app.log.error(error);
    await pool.end();
    process.exit(1);
  }
}

await start();
