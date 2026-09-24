import assert from 'node:assert/strict';
import test from 'node:test';
import { probeD1 } from '../scripts/probe-d1.mjs';

test('upstream SQLite schemas and D1 transaction boundaries', async () => {
  const result = await probeD1();
  for (const migration of Object.values(result.migrations)) {
    assert.equal(migration.failed, undefined, JSON.stringify(migration));
    assert.ok(migration.total > 0);
    assert.equal(migration.applied, migration.total);
  }
  assert.equal(result.interactiveTransactions, 'unsupported');
  assert.equal(result.failedBatchRollback, 'passed');
  assert.equal(result.batchReadOwnWrites, 'passed');
  assert.equal(result.binarySpanPayload, 'passed');
});
