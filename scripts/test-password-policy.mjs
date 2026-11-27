import assert from 'node:assert/strict';
import test from 'node:test';
import { PASSWORD_POLICY, validNewPassword } from '../packages/core/auth/password-policy.ts';

test('new passwords allow exactly six Unicode characters', () => {
  assert.equal(PASSWORD_POLICY.min, 6);
  for (const password of ['r7Q!x9', '密码六个字符', '😀😀😀😀😀😀', ' a b c ']) assert.equal(validNewPassword(password), true);
  for (const password of ['', 'short', '密码五个字', '😀😀😀😀😀']) assert.equal(validNewPassword(password), false);
});

test('UTF-8 byte ceiling and invalid character guards are unchanged', () => {
  assert.equal(PASSWORD_POLICY.maxBytes, 72);
  assert.equal(validNewPassword('x'.repeat(72)), true);
  assert.equal(validNewPassword('界'.repeat(24)), true);
  assert.equal(validNewPassword('😀'.repeat(18)), true);
  assert.equal(validNewPassword('x'.repeat(73)), false);
  assert.equal(validNewPassword('界'.repeat(25)), false);
  assert.equal(validNewPassword('😀'.repeat(19)), false);
  assert.equal(validNewPassword('abc\u0000def'), false);
  assert.equal(validNewPassword('abc\ud800def'), false);
});
