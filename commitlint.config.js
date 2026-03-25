export default {
  extends: ['@commitlint/config-conventional'],
  rules: {
    'type-enum': [
      2,
      'always',
      ['feat', 'fix', 'refactor', 'perf', 'docs', 'test', 'chore', 'ci', 'build', 'style'],
    ],
    'subject-max-length': [2, 'always', 72],
  },
}
