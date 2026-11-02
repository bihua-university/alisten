/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        primary: '#165DFF',
        secondary: '#722ED1',
        dark: '#1E293B',
        light: '#F8FAFC',
      },
      fontFamily: {
        sans: ['-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'Microsoft YaHei', 'sans-serif'],
      },
      fontSize: {
        'ui-xl': 'var(--text-ui-xl)',
        'ui-lg': 'var(--text-ui-lg)',
        'ui-base': 'var(--text-ui-base)',
        'ui-caption': 'var(--text-ui-caption)',
        'ui-sm': 'var(--text-ui-sm)',
        'ui-xs': 'var(--text-ui-xs)',
        'mobile-input-safe': 'var(--text-mobile-input-safe)',
      },
      height: {
        'screen-mobile': 'calc(var(--vh, 1vh) * 100)',
      },
      minHeight: {
        'screen-mobile': 'calc(var(--vh, 1vh) * 100)',
      },
    },
  },
  plugins: [],
}
