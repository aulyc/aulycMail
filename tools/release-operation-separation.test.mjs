import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const makefile = await readFile(new URL('../Makefile', import.meta.url), 'utf8');

function targetBody(name) {
	const match = makefile.match(new RegExp(`^${name}:.*\\n((?:\\t.*\\n)+)`, 'mu'));
	assert.ok(match, `missing Make target: ${name}`);
	return match[1];
}

test('pure release targets never invoke installation targets', () => {
	const testRelease = targetBody('release-test');
	const formalRelease = targetBody('release-formal');
	assert.match(testRelease, /test-release-dmg/u);
	assert.doesNotMatch(testRelease, /install/u);
	assert.match(formalRelease, /release-dmg/u);
	assert.match(formalRelease, /release-backup-push/u);
	assert.doesNotMatch(formalRelease, /install/u);
});

test('combined release-install targets make installation explicit', () => {
	assert.match(targetBody('release-test-install'), /install-test-release-dmg/u);
	assert.match(targetBody('release-formal-install'), /install-release-dmg/u);
	assert.doesNotMatch(targetBody('install-release-dmg'), /release-dmg/u);
	assert.doesNotMatch(targetBody('install-test-release-dmg'), /test-release-dmg/u);
});
