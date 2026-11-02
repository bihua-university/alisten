// eslint.config.mjs
import antfu from '@antfu/eslint-config'

export default antfu({
  vue: false,
  react: true,
  // 由服务端 protocolgen 生成并原样拷贝，不参与风格检查
  ignores: ['src/types/ws-protocol.ts'],
  rules: {
    'no-console': 'off',
    'no-alert': 'off',
    'style/brace-style': ['error', '1tbs'],
  },
  stylistic: {
    'style/brace-style': 'off',
  },
})
