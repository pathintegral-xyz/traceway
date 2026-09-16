import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { Miniflare, convertV4MiniflareOptions } from 'miniflare';

export async function probeD1() {
  const migrations = JSON.parse(execFileSync('python3', [
    fileURLToPath(new URL('./read-migrations.py', import.meta.url)),
  ], { encoding: 'utf8' }));
  const runtime = new Miniflare(convertV4MiniflareOptions({
    modules: true,
    script: 'export default { fetch() { return new Response("local D1 probe"); } };',
    compatibilityDate: '2026-09-16',
    d1Databases: ['MAIN', 'TELEMETRY', 'TRANSACTIONS'],
  }));
  const results = { runtime: 'local workerd D1', migrations: {} };
  try {
    for (const [binding, files] of Object.entries(migrations)) {
      const database = await runtime.getD1Database(binding);
      const applied = [];
      for (const file of files) {
        try {
          await database.batch(file.statements.map(sql => database.prepare(sql)));
          applied.push(file.name);
        } catch (error) {
          results.migrations[binding] = { applied: applied.length, failed: file.name, error: error.message };
          break;
        }
      }
      results.migrations[binding] ??= { applied: applied.length, total: files.length };
    }

    const database = await runtime.getD1Database('TRANSACTIONS');
    await database.prepare('CREATE TABLE items (id INTEGER PRIMARY KEY, value TEXT NOT NULL UNIQUE)').run();
    await assert.rejects(database.prepare('BEGIN TRANSACTION').run());
    results.interactiveTransactions = 'unsupported';

    await assert.rejects(database.batch([
      database.prepare('INSERT INTO items VALUES (1, ?)').bind('duplicate'),
      database.prepare('INSERT INTO items VALUES (2, ?)').bind('duplicate'),
    ]));
    assert.equal(await database.prepare('SELECT count(*) FROM items').first('count(*)'), 0);
    results.failedBatchRollback = 'passed';

    const batch = await database.batch([
      database.prepare('INSERT INTO items VALUES (1, ?)').bind('first'),
      database.prepare('SELECT value FROM items WHERE id = 1'),
    ]);
    assert.equal(batch[1].results[0].value, 'first');
    results.batchReadOwnWrites = 'passed';
    return results;
  } finally {
    await runtime.dispose();
  }
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const results = await probeD1();
  console.log(JSON.stringify(results, null, 2));
  if (Object.values(results.migrations).some(result => result.failed)) process.exitCode = 1;
}
