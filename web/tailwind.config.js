/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        background: '#090d16',
        card: '#111827',
        cardBorder: '#1f2937',
        statusUp: '#10b981',
        statusDown: '#f43f5e',
        statusWarn: '#f59e0b',
        statusPending: '#6b7280',
      }
    },
  },
  plugins: [],
}
