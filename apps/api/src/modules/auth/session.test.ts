import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionToken, getBearerToken, verifySessionToken } from './session.js';

test('createSessionToken signs claims and verifySessionToken returns them', () => {
  process.env.SESSION_SECRET = 'test-secret';
  const token = createSessionToken({
    userId: 'user-1',
    workspaceId: 'workspace-1',
    role: 'owner',
  });

  const claims = verifySessionToken(token);
  assert.equal(claims?.userId, 'user-1');
  assert.equal(claims?.workspaceId, 'workspace-1');
  assert.equal(claims?.role, 'owner');
});

test('verifySessionToken rejects tampered tokens', () => {
  process.env.SESSION_SECRET = 'test-secret';
  const token = createSessionToken({
    userId: 'user-1',
    workspaceId: 'workspace-1',
    role: 'owner',
  });
  const parts = token.split('.');
  const tampered = `${parts[0]}.${Buffer.from(
    JSON.stringify({ userId: 'user-2', workspaceId: 'workspace-1', role: 'owner', exp: 9999999999 })
  ).toString('base64url')}.${parts[2]}`;

  assert.equal(verifySessionToken(tampered), null);
});

test('verifySessionToken rejects expired tokens', () => {
  process.env.SESSION_SECRET = 'test-secret';
  const token = createSessionToken(
    {
      userId: 'user-1',
      workspaceId: 'workspace-1',
      role: 'owner',
    },
    -1
  );

  assert.equal(verifySessionToken(token), null);
});

test('verifySessionToken rejects malformed tokens without throwing', () => {
  process.env.SESSION_SECRET = 'test-secret';

  assert.equal(verifySessionToken('not.a.valid.token'), null);
  assert.equal(verifySessionToken('not-json.not-json.signature'), null);
  assert.equal(verifySessionToken('too.many.parts.in.token'), null);
});

test('createSessionToken requires SESSION_SECRET in production', () => {
  const originalNodeEnv = process.env.NODE_ENV;
  const originalSessionSecret = process.env.SESSION_SECRET;
  process.env.NODE_ENV = 'production';
  delete process.env.SESSION_SECRET;

  try {
    assert.throws(
      () =>
        createSessionToken({
          userId: 'user-1',
          workspaceId: 'workspace-1',
          role: 'owner',
        }),
      /SESSION_SECRET must be set in production/
    );
  } finally {
    if (originalNodeEnv === undefined) delete process.env.NODE_ENV;
    else process.env.NODE_ENV = originalNodeEnv;

    if (originalSessionSecret === undefined) delete process.env.SESSION_SECRET;
    else process.env.SESSION_SECRET = originalSessionSecret;
  }
});

test('getBearerToken parses bearer authorization headers', () => {
  assert.equal(getBearerToken('Bearer abc123'), 'abc123');
  assert.equal(getBearerToken('Basic abc123'), null);
  assert.equal(getBearerToken(undefined), null);
});
