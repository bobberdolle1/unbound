/*
 * Unbound for WebOS — Main entry point
 * Uses Enact framework with Moonstone theme for TV-optimized D-pad navigation
 */

import {createRoot} from 'react-dom/client';
import App from './src/App';

const root = createRoot(document.getElementById('root'));
root.render(<App />);
