/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        primary: {
          DEFAULT: '#1E3A8A',
          dark: '#3B82F6',
        },
        background: {
          light: '#F8FAFC',
          dark: '#0F172A',
        },
        accent: {
          DEFAULT: '#7C3AED',
          dark: '#A78BFA',
        },
      },
      backdropBlur: {
        glass: '16px',
      },
    },
  },
  plugins: [],
}
