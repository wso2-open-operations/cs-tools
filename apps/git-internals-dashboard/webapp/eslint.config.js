import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import security from 'eslint-plugin-security'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
      security.configs.recommended,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    rules: {
      // Every hit in this codebase indexes a small constant lookup record
      // (SLA_STATE_COLOR[state], BUCKET_TITLES[bucket], etc.) by a value
      // TypeScript already constrains to a closed union — the rule can't see
      // that constraint and flags it the same as an unvalidated obj[userInput].
      'security/detect-object-injection': 'off',
    },
  },
])
