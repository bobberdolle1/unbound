import typescript from "@rollup/plugin-typescript";
import resolve from "@rollup/plugin-node-resolve";
import commonjs from "@rollup/plugin-commonjs";
import css from "rollup-plugin-import-css";

export default {
  input: "src/index.tsx",
  plugins: [
    typescript({ 
      tsconfig: "./tsconfig.json",
      jsx: "react",
      jsxFactory: "h",
      jsxFragmentFactory: "Fragment",
    }),
    resolve(),
    commonjs(),
    css(),
  ],
  output: {
    file: "dist/index.js",
    format: "iife",
    globals: {
      react: "SP_REACT",
      "react-dom": "SP_REACTDOM",
      "@decky/ui": "DeckyUI",
    },
  },
  external: ["react", "react-dom", "@decky/ui"],
};
