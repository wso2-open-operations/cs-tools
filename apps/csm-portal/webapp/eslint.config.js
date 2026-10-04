import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
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
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    rules: {
      // The `const { omitted, ...rest } = obj` idiom is how fields are dropped
      // from an object here. Both eslint core and typescript-eslint default
      // `ignoreRestSiblings` to false, so the deliberately-unused binding is
      // reported as an error; opting in is the option's intended use.
      '@typescript-eslint/no-unused-vars': ['error', { ignoreRestSiblings: true }],
      // Markup and navigation sinks. Warn (not error) while the remaining
      // call sites are migrated; flip to 'error' once the count reaches zero.
      'no-restricted-syntax': [
        'warn',
        {
          selector: "AssignmentExpression[left.type='MemberExpression'][left.property.name='innerHTML']",
          message:
            'Do not assign innerHTML. Build DOM nodes, or render sanitised markup through renderTrustedHtml (@utils/renderTrustedHtml).',
        },
        {
          selector:
            "CallExpression[callee.object.name='window'][callee.property.name='open'][arguments.length<3]",
          message:
            'Do not call window.open without a features argument. Use openExternalUrl (@utils/openExternalUrl) for external URLs, or pass "noopener,noreferrer" for same-origin paths.',
        },
      ],
    },
  },
])
