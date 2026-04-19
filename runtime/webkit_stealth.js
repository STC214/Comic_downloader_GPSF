/* WebKit Stealth Patch v1.0
   Optimized specifically for Playwright WebKit on Windows.
*/
(() => {
    // 1. Remove the WebDriver marker.
    // navigator.webdriver in WebKit can often be overridden directly, but we use DefineProperty for safety.
    try {
        Object.defineProperty(navigator, 'webdriver', {
            get: () => false,
            enumerable: true,
            configurable: false
        });
    } catch (e) {}

    // 2. Mask language and platform signals.
    // WebKit on Windows can sometimes expose inconsistent platform information.
    try {
        Object.defineProperty(navigator, 'languages', {
            get: () => ['zh-CN', 'zh', 'en-US', 'en'],
            configurable: true
        });
        Object.defineProperty(navigator, 'platform', {
            get: () => 'MacIntel', // Works best with a Safari UA to resemble macOS.
            configurable: true
        });
    } catch (e) {}

    // 3. Remove automation residue.
    try {
        delete window.__playwright;
        delete window.__pw_cleanup;
        delete window.__PW_inspect;
    } catch (e) {}

    // 4. Prevent WebKit-specific automation checks with a simple permission mask.
    if (window.Notification && Notification.permission === 'granted') {
        // Keep the default state so permissions do not look preconfigured by automation.
    }
})();
