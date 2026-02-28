/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        obsidian: '#08090d',
        surface: {
          DEFAULT: '#0c0d14',
          raised: '#12141e',
          hover: '#181b28',
        },
        edge: {
          DEFAULT: '#1e2235',
          hover: '#2a2f48',
        },
        accent: {
          DEFAULT: '#00ffaa',
          dim: '#00cc88',
          muted: 'rgba(0,255,170,0.08)',
        },
        danger: {
          DEFAULT: '#ff4f5e',
          muted: 'rgba(255,79,94,0.12)',
        },
        warn: {
          DEFAULT: '#ffb224',
          muted: 'rgba(255,178,36,0.12)',
        },
        ok: {
          DEFAULT: '#3dd68c',
          muted: 'rgba(61,214,140,0.12)',
        },
        info: {
          DEFAULT: '#52a8ff',
          muted: 'rgba(82,168,255,0.12)',
        },
        ghost: {
          DEFAULT: '#53566e',
          faint: '#2e3148',
        },
      },
      fontFamily: {
        display: ['Chakra Petch', 'sans-serif'],
        body: ['Outfit', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
      animation: {
        'fade-up': 'fadeUp 0.5s ease-out both',
        'fade-in': 'fadeIn 0.4s ease-out both',
        'glow-pulse': 'glowPulse 2s ease-in-out infinite',
        'slide-in': 'slideIn 0.3s ease-out both',
        'scan': 'scan 4s linear infinite',
      },
      keyframes: {
        fadeUp: {
          '0%': { opacity: '0', transform: 'translateY(12px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        glowPulse: {
          '0%, 100%': { opacity: '0.4' },
          '50%': { opacity: '1' },
        },
        slideIn: {
          '0%': { opacity: '0', transform: 'translateX(-8px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' },
        },
        scan: {
          '0%': { transform: 'translateY(-100%)' },
          '100%': { transform: 'translateY(100%)' },
        },
      },
    },
  },
  plugins: [],
}
