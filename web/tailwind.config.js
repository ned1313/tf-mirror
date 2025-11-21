/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        primary: '#5c4ee5',
        'primary-dark': '#4a3ec7',
      }
    },
  },
  plugins: [],
}
