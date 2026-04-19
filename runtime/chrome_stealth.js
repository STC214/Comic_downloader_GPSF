/* Chrome Stealth Final 加固版 
   集成功能：WebDriver 深度抹除 + 原型鏈清理 + isTrusted 劫持 + Shadow DOM 穿透點擊 + 窗口屬性偽裝
*/
(() => {
    // 1. 核心：深度抹除 WebDriver 與自動化特徵
    const maskWebDriver = () => {
        try {
            const proto = Navigator.prototype;
            
            // 徹底刪除原型上的 webdriver
            delete proto.webdriver;
            
            Object.defineProperty(proto, 'webdriver', {
                get: () => undefined, 
                enumerable: false,
                configurable: false
            });

            // 偽裝 window.outerHeight/Width (防止 Headless 檢測)
            // 如果是在無頭模式下，outerHeight 通常等於 innerHeight，這是不正常的
            if (window.outerHeight <= window.innerHeight) {
                const decorationHeight = 85; // 模擬標題欄和地址欄高度
                Object.defineProperty(window, 'outerHeight', {
                    get: () => window.innerHeight + decorationHeight,
                    configurable: true
                });
            }

            if (navigator.hasOwnProperty('webdriver')) {
                delete navigator.webdriver;
            }
        } catch (e) {}
    };

    // 2. 模擬真實 Chrome 專有對象
    const maskChrome = () => {
        window.chrome = {
            app: {
                isInstalled: false,
                InstallState: { DISABLED: "disabled", INSTALLED: "installed", NOT_INSTALLED: "not_installed" },
                RunningState: { CANNOT_RUN: "cannot_run", READY_TO_RUN: "ready_to_run", RUNNING: "running" },
            },
            loadTimes: () => ({
                requestTime: Date.now() / 1000,
                startLoadTime: Date.now() / 1000,
                commitLoadTime: Date.now() / 1000,
                finishDocumentLoadTime: Date.now() / 1000,
                firstPaintTime: Date.now() / 1000,
                finishLoadTime: Date.now() / 1000,
                wasFetchedViaSpdy: true,
                wasNpnNegotiated: true,
                wasAlternateProtocolAvailable: false,
                connectionInfo: "h2"
            }),
            csi: () => ({ startE: Date.now(), onloadT: Date.now(), pageT: 2000, tran: 15 }),
        };
    };

    // 3. 插件與硬體原型鏈深度偽裝 (解決 PluginArray failed)
    const maskHardware = () => {
        const originalQuery = window.navigator.permissions.query;
        window.navigator.permissions.query = (parameters) => (
            parameters.name === 'notifications' ?
                Promise.resolve({ state: Notification.permission }) :
                originalQuery(parameters)
        );

        const mockPlugins = [
            { name: 'PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
            { name: 'Chrome PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
            { name: 'Chromium PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
            { name: 'Microsoft Edge PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
            { name: 'WebKit built-in PDF', filename: 'internal-pdf-viewer', description: 'Portable Document Format' }
        ];

        const pluginArray = {};
        mockPlugins.forEach((p, i) => {
            const plugin = {
                name: p.name,
                filename: p.filename,
                description: p.description,
                length: 0,
                item: () => null,
                namedItem: () => null
            };
            Object.setPrototypeOf(plugin, Plugin.prototype);
            pluginArray[i] = plugin;
            pluginArray[p.name] = plugin;
        });

        Object.defineProperties(pluginArray, {
            length: { value: mockPlugins.length },
            item: { value: (index) => pluginArray[index] || null },
            namedItem: { value: (name) => pluginArray[name] || null },
            refresh: { value: () => {} }
        });

        Object.setPrototypeOf(pluginArray, PluginArray.prototype);

        Object.defineProperty(navigator, 'plugins', {
            get: () => pluginArray,
            configurable: true
        });

        Object.defineProperty(navigator, 'languages', { get: () => ['zh-CN', 'zh', 'en-US', 'en'], configurable: true });
    };

    // 4. 事件可信度偽裝 (劫持 isTrusted)
    const maskEvents = () => {
        const originalAddEventListener = Element.prototype.addEventListener;
        Element.prototype.addEventListener = function (type, listener, options) {
            const patchedListener = function (event) {
                if (event instanceof Event) {
                    try {
                        Object.defineProperty(event, 'isTrusted', { value: true, enumerable: true });
                    } catch (e) {}
                }
                return typeof listener === 'function' ? listener.call(this, event) : listener.handleEvent(event);
            };
            return originalAddEventListener.call(this, type, patchedListener, options);
        };
    };

    // 5. 穿透型 Cloudflare Turnstile 自動點擊器 (核心加固：支持 Shadow DOM)
    const cloudflareClicker = () => {
        const findAndClick = (root) => {
            // 擴展選擇器
            const selectors = [
                'input[type=checkbox]', 
                '#challenge-stage input', 
                '.ctp-checkbox-label',
                '#mark-as-read'
            ];
            
            for (let s of selectors) {
                const node = root.querySelector(s);
                // 必須是可見元素
                if (node && node.offsetWidth > 0 && node.offsetHeight > 0) {
                    node.click();
                    return true;
                }
            }

            // 遞歸進入 Shadow Root (CF 驗證碼的核心藏匿地)
            const allElements = root.querySelectorAll('*');
            for (let el of allElements) {
                if (el.shadowRoot && findAndClick(el.shadowRoot)) {
                    return true;
                }
            }
            return false;
        };

        // 提高檢查頻率至 500ms
        setInterval(() => {
            if (!findAndClick(document)) {
                // 嘗試處理同源 iframe
                document.querySelectorAll('iframe').forEach(ifr => {
                    try {
                        if (ifr.contentDocument) findAndClick(ifr.contentDocument);
                    } catch (e) {
                        // 跨域 iframe 忽略，交由行為模擬處理
                    }
                });
            }
        }, 500);
    };

    // 6. 清理 Playwright 殘留特徵
    const cleanup = () => {
        try {
            delete window.__playwright;
            delete window.__pw_cleanup;
            delete window.__PW_inspect;
        } catch (e) {}
    };

    // 執行補丁
    maskWebDriver();
    maskChrome();
    maskHardware();
    maskEvents();
    cloudflareClicker();
    cleanup();
})();