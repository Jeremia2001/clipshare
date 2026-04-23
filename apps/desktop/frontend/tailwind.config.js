/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Xbox-inspired palette: near-black backgrounds, Xbox green accents
        forest: {
          50: '#ecfce8',
          100: '#d0f8c8',
          200: '#a2f094',
          300: '#68e260',
          400: '#4dcf43',
          500: '#52b043',  // Xbox green
          600: '#2e8c1e',  // dark Xbox green for primary actions
          700: '#1e6014',
          800: '#143e0c',
          900: '#0c2408',
          950: '#070a08',  // near-black background
        },
        earth: {
          50: '#fef2f0',
          100: '#fcddd8',
          200: '#f9b8b0',
          300: '#f48a80',
          400: '#ec5c50',
          500: '#dc3028',
          600: '#bc2020',
          700: '#941818',
          800: '#6c1010',
          900: '#480c0c',
          950: '#280606',
        },
        sand: {
          50: '#f4f4f6',
          100: '#e0e0e8',  // primary text — neutral silver
          200: '#c4c4ce',
          300: '#a8a8b8',
          400: '#8c8ca0',
          500: '#70707c',  // secondary text — cool gray
          600: '#585868',
          700: '#424252',
          800: '#30303e',
          900: '#20202c',
          950: '#141420',
        },
        moss: {
          50: '#e8fde6',
          100: '#ccfac8',
          200: '#96f492',
          300: '#60e858',
          400: '#52b043',  // Xbox green
          500: '#3d8832',
          600: '#2a5c22',
          700: '#1c3c18',
          800: '#122810',
          900: '#0c1a0a',
          950: '#060c04',
        },
      },
    },
  },
  plugins: [],
}