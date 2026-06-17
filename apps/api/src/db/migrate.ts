import { readFile, readdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { Client } from 'pg';
import { loadEnvFromProjectRoot } from '../lib/env.js';

loadEnvFromProjectRoot();

const databaseUrl = process.env.DATABASE_URL ?? 'postgres://localhost:5432/dealsignal';
const migrationsDir = resolve(import.meta.dirname, '../../drizzle/migrations');

async function ensureMigrationsTable(client: Client) {
  await client.query(`
    CREATE TABLE IF NOT EXISTS schema_migrations (
      filename TEXT PRIMARY KEY,
      applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
    );
  `);
}

async function getAppliedMigrations(client: Client): Promise<Set<string>> {
  const result = await client.query<{ filename: string }>(
    'SELECT filename FROM schema_migrations ORDER BY filename'
  );
  return new Set(result.rows.map((row) => row.filename));
}

async function getMigrationFiles(): Promise<string[]> {
  const entries = await readdir(migrationsDir);
  return entries
    .filter((name) => name.endsWith('.sql'))
    .sort((a, b) => a.localeCompare(b));
}

async function run() {
  const client = new Client({ connectionString: databaseUrl });
  await client.connect();

  try {
    await client.query('BEGIN');
    await ensureMigrationsTable(client);
    const applied = await getAppliedMigrations(client);
    const files = await getMigrationFiles();

    if (files.length === 0) {
      console.log('No migration files found.');
      await client.query('COMMIT');
      return;
    }

    let appliedCount = 0;
    for (const file of files) {
      if (applied.has(file)) {
        console.log(`Skipping already applied migration: ${file}`);
        continue;
      }

      const path = resolve(migrationsDir, file);
      const sql = await readFile(path, 'utf-8');
      console.log(`Applying migration: ${file}`);
      await client.query(sql);
      await client.query('INSERT INTO schema_migrations (filename) VALUES ($1)', [file]);
      appliedCount++;
    }

    await client.query('COMMIT');
    console.log(`Migrations complete. Applied ${appliedCount} new migration(s).`);
  } catch (error) {
    await client.query('ROLLBACK').catch(() => undefined);
    throw error;
  } finally {
    await client.end();
  }
}

run().catch((error) => {
  console.error('Migration failed:', error);
  process.exit(1);
});
