/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './src/**/*.{js,jsx,ts,tsx,html}',
    './popup.html',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Doodle Jump Minimalism palette
        doodle: {
          bg: '#f7f5f0',
          surface: '#ffffff',
          primary: '#5cb85c',
          primaryHover: '#4cae4c',
          accent: '#f0ad4e',
          text: '#333333',
          textMuted: '#888888',
          border: '#e0ddd5',
        },
        // Modern Dark palette
        dark: {
          bg: '#1a1a2e',
          surface: '#16213e',
          primary: '#0f3460',
          primaryHover: '#1a4a7a',
          accent: '#e94560',
          text: '#eaeaea',
          textMuted: '#a0a0b0',
          border: '#2a2a4a',
        },
      },
      fontFamily: {
        mono: ['"JetBrains Mono"', 'Fira Code', 'monospace'],
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'bounce-subtle': 'bounce-subtle 2s ease-in-out infinite',
      },
      keyframes: {
        'bounce-subtle': {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-4px)' },
        },
      },
    },
  },
  plugins: [],
};
