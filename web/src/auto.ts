/**
 * Auto-initialization for Live v2 Islands Architecture.
 *
 * This module:
 * - Registers the LiveIsland custom element
 * - Auto-registers hooks from window.Hooks
 * - No manual initialization needed - <live-island> elements work automatically
 *
 * Usage:
 * 1. Include this script in your HTML: <script src="auto.js"></script>
 * 2. Optionally define hooks before the script loads:
 *    <script>
 *      window.Hooks = {
 *        MyComponent: {
 *          mounted() { console.log('Mounted:', this.el); }
 *        }
 *      };
 *    </script>
 * 3. Add islands to your HTML:
 *    <live-island type="counter" id="counter-1">
 *      <!-- Island content -->
 *    </live-island>
 */

import { registerLiveIsland } from "./island";
import { autoRegisterHooks } from "./hooks";

// Register the LiveIsland custom element
// This makes <live-island> available immediately
registerLiveIsland();

// Auto-register any hooks defined on window.Hooks
// This runs on module load, before DOMContentLoaded
autoRegisterHooks();

console.debug('Live v2: auto-initialization complete');

// Note: No DOMContentLoaded listener needed!
// Custom elements are automatically initialized when parsed by the browser.
