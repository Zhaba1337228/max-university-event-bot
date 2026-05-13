import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./lib/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        bg: "#0b0d12",
        surface: "#121620",
        muted: "#1c2230",
        border: "#252d3e",
        accent: "#7c5cff",
        accentHover: "#6346ff",
        success: "#22c55e",
        danger: "#ef4444",
        text: "#e6e8ee",
        subtle: "#8a93a6",
      },
      fontFamily: {
        sans: ["system-ui", "-apple-system", "Segoe UI", "Roboto", "sans-serif"],
      },
    },
  },
  plugins: [],
};

export default config;
