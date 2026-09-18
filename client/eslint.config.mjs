import eslint from '@eslint/js';
import babelParser from '@babel/eslint-parser';
import globals from 'globals';
import eslintPluginVue from 'eslint-plugin-vue';

const babelTypeScriptOptions = {
  requireConfigFile: false,
  babelOptions: {
    plugins: ['@babel/plugin-syntax-typescript'],
  },
};

export default [
  {
    ignores: ['dist/**', 'node_modules/**', 'wailsjs/**', '**/*.d.ts'],
  },
  eslint.configs.recommended,
  ...eslintPluginVue.configs['flat/essential'],
  {
    files: ['**/*.ts'],
    languageOptions: {
      globals: globals.browser,
      parser: babelParser,
      parserOptions: babelTypeScriptOptions,
    },
    rules: {
      'no-undef': 'off',
      'no-unused-vars': 'off',
    },
  },
  {
    files: ['**/*.vue'],
    languageOptions: {
      globals: globals.browser,
      parserOptions: {
        parser: babelParser,
        ...babelTypeScriptOptions,
      },
    },
    rules: {
      'no-undef': 'off',
      'no-unused-vars': 'off',
    },
  },
  {
    rules: {
      'vue/multi-word-component-names': 'off',
    },
  },
];
