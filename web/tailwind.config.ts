import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./lib/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      screens: {
        // sm: 640px (default), md: 768px (default), lg: 1024px (default), xl: 1280px (default)
        xs: "480px",
      },
      colors: {
        // Тёмная палитра, выверенная под WCAG AA на основном тексте.
        bg: "#0b0d12",            // фон страницы
        surface: "#141926",       // карточки, шапка
        surfaceAlt: "#1a2030",    // hover-состояние, чуть светлее
        muted: "#1f2638",         // input/badge фон
        border: "#2a3346",        // более заметная граница
        accent: "#7c5cff",        // primary CTA
        accentHover: "#6a4fff",
        accentSoft: "#7c5cff14",  // ~8% — для подсветки активного состояния
        success: "#22c55e",
        warn: "#f59e0b",
        danger: "#ef4444",
        text: "#f1f4fb",          // основной текст (контраст ~14:1 на bg)
        subtle: "#a1aac0",        // вторичный текст (контраст ~6:1 на bg, AA для regular ≥18px)
      },
      fontFamily: {
        sans: ["system-ui", "-apple-system", "Segoe UI", "Roboto", "sans-serif"],
      },
      boxShadow: {
        card: "0 1px 2px rgba(0,0,0,0.4), 0 1px 3px rgba(0,0,0,0.25)",
        elevated: "0 6px 24px rgba(0,0,0,0.45)",
      },
    },
  },
  plugins: [],
};

export default config;
