/// <reference types="vite/client" />

import 'react';

declare module 'react' {
  interface CSSProperties {
    '--wails-draggable'?: 'drag' | 'no-drag' | string;
  }
}
