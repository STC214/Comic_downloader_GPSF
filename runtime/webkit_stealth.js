/* WebKit Stealth Patch v1.0 
   专门针对 Playwright WebKit (Windows) 优化
*/
(() => {
    // 1. 抹除 WebDriver 标记
    // WebKit 中 navigator.webdriver 通常是可直接覆盖的，但为了保险我们使用 DefineProperty
    try {
        Object.defineProperty(navigator, 'webdriver', {
            get: () => false,
            enumerable: true,
            configurable: false
        });
    } catch (e) {}

    // 2. 伪装语言和平台
    // WebKit 在 Windows 上运行时，有时会暴露不一致的平台信息
    try {
        Object.defineProperty(navigator, 'languages', {
            get: () => ['zh-CN', 'zh', 'en-US', 'en'],
            configurable: true
        });
        Object.defineProperty(navigator, 'platform', {
            get: () => 'MacIntel', // 配合 Safari UA，伪装成 Mac 效果最好
            configurable: true
        });
    } catch (e) {}

    // 3. 抹除自动化残留
    try {
        delete window.__playwright;
        delete window.__pw_cleanup;
        delete window.__PW_inspect;
    } catch (e) {}

    // 4. 防止 WebKit 特有的自动化检测 (简单的权限伪装)
    if (window.Notification && Notification.permission === 'granted') {
        // 保持默认状态，避免被检测到权限已被自动化预设
    }
})();