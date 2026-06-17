import assert from 'node:assert/strict';
import test from 'node:test';
import { hashPassword, verifyPassword } from './password.js';

test('hashPassword creates salted hashes and verifyPassword validates them', () => {
  const password = 'correct horse battery staple';
  const firstHash = hashPassword(password);
  const secondHash = hashPassword(password);

  assert.notEqual(firstHash, password);
  assert.notEqual(firstHash, secondHash);
  assert.equal(verifyPassword(password, firstHash), true);
  assert.equal(verifyPassword('wrong password', firstHash), false);
});

test('verifyPassword rejects malformed hashes', () => {
  assert.equal(verifyPassword('password', 'not-a-valid-hash'), false);
  assert.equal(verifyPassword('password', 'scrypt$missing-hash'), false);
});
