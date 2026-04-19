/* Firefox Stealth Patch v1.0 */
(() => {
    // 1. 彻底抹除 navigator.webdriver
    const newProto = Object.getPrototypeOf(navigator);
    delete newProto.webdriver;
    Object.defineProperty(navigator, 'webdriver', { 
        get: () => undefined,
        configurable: true 
    });

    // 2. 伪装 Firefox 特有的 BuildID (改为标准发布版日期)
    Object.defineProperty(navigator, 'buildID', { 
        get: () => '20181001010101',
        configurable: true 
    });

    // 3. 补全 Firefox 默认应有的 Plugins 列表 (防止因列表为空被识别)
    const mockPlugins = [
        { name: 'PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
        { name: 'Chrome PDF Viewer', filename: 'internal-pdf-viewer', description: 'Google Chrome PDF Viewer' }
    ];
    Object.defineProperty(navigator, 'plugins', {
        get: () => {
            const arr = mockPlugins;
            arr.item = (i) => arr[i];
            arr.namedItem = (name) => arr.find(p => p.name === name);
            arr.refresh = () => {};
            return arr;
        },
        configurable: true
    });

    // 4. 伪装硬件并发数与语言
    Object.defineProperty(navigator, 'hardwareConcurrency', { get: () => 8 });
    Object.defineProperty(navigator, 'languages', { get: () => ['zh-CN', 'zh', 'en-US', 'en'] });

    // 5. 修复权限检测 (解决自动化环境下 Permissions 返回 'denied' 的问题)
    const originalQuery = window.navigator.permissions.query;
    window.navigator.permissions.query = (parameters) => (
        parameters.name === 'notifications' ?
        Promise.resolve({ state: Notification.permission }) :
        originalQuery(parameters)
    );

    // 6. 抹除 Playwright 注入痕迹
    const cleanUp = () => {
        delete window.__playwright;
        delete window.__pw_cleanup;
        delete window.__PW_inspect;
    };
    cleanUp();
    
    // 每秒清理一次，防止动态注入检测
    setInterval(cleanUp, 1000);
})();