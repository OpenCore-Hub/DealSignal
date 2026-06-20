import { drizzle } from 'drizzle-orm/node-postgres';
import { Pool } from 'pg';
import { loadEnvFromProjectRoot } from '../lib/env.js';
import * as schema from './schema.js';

loadEnvFromProjectRoot();

const databaseUrl = process.env.DATABASE_URL ?? 'postgres://localhost:5432/dealsignal';

export const pool = new Pool({
  connectionString: databaseUrl,
});

export const db = drizzle(pool, { schema });

export type Database = typeof db;
