import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { HELPER_INSTRUCTIONS } from '../packages/views/onboarding/templates/helper-instructions.ts';

test('default MEPER Agent prompts consistently describe the branded CLI', () => {
  for (const [lang, prompt] of Object.entries(HELPER_INSTRUCTIONS)) {
    assert.match(prompt, /`meperstudio --help`/, lang);
    assert.doesNotMatch(prompt, /\bsilieco\b|SILIECO_/, lang);
  }
});

test('legacy server fallback stays aligned with the English default prompt', () => {
  const source = readFileSync(new URL('../core/internal/handler/onboarding_shim.go', import.meta.url), 'utf8');
  const expression = source.split('const onboardingAssistantInstructions = ')[1].split('\n\nconst onboardingTaskDescription')[0];
  const actual = expression.replaceAll('` + "`" + `', '`').slice(1, -1);
  assert.equal(actual, HELPER_INSTRUCTIONS.en);
});
