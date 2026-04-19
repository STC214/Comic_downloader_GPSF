/* Firefox Stealth Final Version v2.1
   针对 Cloudflare (CF) 与主流指纹检测站点优化
*/
(() => {
    // 1. 深度拦截 WebDriver (采用原型链锁定方案)
    const maskWebDriver = () => {
        try {
            const proto = Navigator.prototype;
            // 彻底删除原生定义并锁定
            delete proto.webdriver;
            Object.defineProperty(proto, 'webdriver', {
                get: () => false,
                enumerable: true,
                configurable: false
            });
            // 清理直接挂在 navigator 上的属性
            if (Object.getOwnPropertyDescriptor(navigator, 'webdriver')) {
                delete navigator.webdriver;
            }
        } catch (e) {}
    };

    // 2. 插件数组伪装 (模拟真实 Firefox 的 PluginArray 结构)
    const maskPlugins = () => {
        try {
            const mockPlugins = [
                { name: 'PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
                { name: 'Chrome PDF Viewer', filename: 'internal-pdf-viewer', description: 'Google Chrome PDF Viewer' }
            ];

            const fakePlugins = Object.create(PluginArray.prototype);
            
            mockPlugins.forEach((p, i) => {
                const plugin = Object.create(Plugin.prototype);
                Object.defineProperties(plugin, {
                    name: { get: () => p.name, enumerable: true },
                    filename: { get: () => p.filename, enumerable: true },
                    description: { get: () => p.description, enumerable: true }
                });
                fakePlugins[i] = plugin;
                fakePlugins[p.name] = plugin;
            });

            Object.defineProperties(fakePlugins, {
                length: { get: () => mockPlugins.length, enumerable: true },
                item: { value: (i) => fakePlugins[i], writable: false, enumerable: true },
                namedItem: { value: (n) => fakePlugins[n], writable: false, enumerable: true },
                refresh: { value: () => {}, writable: false, enumerable: true }
            });

            // 整体覆盖
            Object.defineProperty(navigator, 'plugins', {
                get: () => fakePlugins,
                enumerable: true,
                configurable: true
            });
        } catch (e) {}
    };

    // 3. 基础环境伪装 (语言、平台、硬件并发)
    const maskEnvironment = () => {
        try {
            Object.defineProperty(navigator, 'languages', { get: () => ['zh-CN', 'zh', 'en-US', 'en'], configurable: true });
            Object.defineProperty(navigator, 'platform', { get: () => 'Win32', configurable: true });
            Object.defineProperty(navigator, 'hardwareConcurrency', { get: () => 8, configurable: true });
            Object.defineProperty(navigator, 'buildID', { get: () => '20181001010101', configurable: true });
        } catch (e) {}
    };

    // 4. 清理 Playwright 和自动化残留标记
    const cleanAutomationTraces = () => {
        try {
            delete window.__playwright;
            delete window.__pw_cleanup;
            delete window.__PW_inspect;
            // 模拟 Chrome 对象不存在（Firefox 原生状态）
            delete window.chrome; 
        } catch (e) {}
    };

    // 顺序执行
    maskWebDriver();
    maskPlugins();
    maskEnvironment();
    cleanAutomationTraces();

    // 控制台静默（不打印 Stealth patch 信息，防止被检测 console.log）
})();