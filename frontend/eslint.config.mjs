// eslint.config.mjs
import antfu from '@antfu/eslint-config'

export default antfu({
  vue: false,
  react: false,
  rules: {
    'no-console': 'off',
    'no-alert': 'off',
    'style/brace-style': ['error', '1tbs'],
  },
  stylistic: {
    'style/brace-style': 'off',
  },
})
