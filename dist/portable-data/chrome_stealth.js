/* Chrome Stealth Final Hardened Version
   針對 CreepJS 高強度檢測與 Pixelscan 指紋一致性進行優化
*/
(() => {
    // 1. 深度抹除 WebDriver 特徵
    const maskWebDriver = () => {
        const proto = Navigator.prototype;
        // 刪除原始屬性
        delete proto.webdriver;
        
        // 使用 Proxy 模擬正常瀏覽器的行為（返回 undefined 且不被察覺）
        Object.defineProperty(proto, 'webdriver', {
            get: () => undefined,
            enumerable: false,
            configurable: false
        });
    };

    // 2. 模擬 Chrome 專有對象與 Runtime
    const maskChrome = () => {
        // 許多自動化檢測會檢查 window.chrome 是否存在
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
                finishLoadTime: Date.now() / 1000,
                firstPaintTime: Date.now() / 1000,
                firstPaintAfterLoadTime: 0,
                navigationType: "Other",
                wasFetchedViaSpdy: true,
                wasNpnNegotiated: true,
                wasAlternateProtocolAvailable: false,
                connectionInfo: "h2",
            }),
            csi: () => ({
                startE: Date.now(),
                onloadT: Date.now(),
                pageT: 0,
                tran: 15,
            }),
            runtime: {
                OnInstalledReason: { CHROME_UPDATE: "chrome_update", INSTALL: "install", SHARED_MODULE_UPDATE: "shared_module_update", UPDATE: "update" },
                OnRestartRequiredReason: { APP_UPDATE: "app_update", OS_UPDATE: "os_update", PERIODIC: "periodic" },
                PlatformArch: { ARM: "arm", ARM64: "arm64", MIPS: "mips", MIPS64: "mips64", X86_32: "x86-32", X86_64: "x86-64" },
                PlatformNaclArch: { ARM: "arm", MIPS: "mips", MIPS64: "mips64", X86_32: "x86-32", X86_64: "x86-64" },
                PlatformOs: { ANDROID: "android", CROS: "cros", LINUX: "linux", MAC: "mac", OPENBSD: "openbsd", WIN: "win" },
                RequestUpdateCheckStatus: { NO_UPDATE: "no_update", THROTTLED: "throttled", UPDATE_AVAILABLE: "update_available" },
            }
        };
    };

    // 3. 修正硬件參數（對齊 CreepJS 截圖數據）
    const maskHardware = () => {
        // 模擬 8 核 CPU 與 8GB 內存，這是目前最常見的成人指紋配置
        Object.defineProperty(navigator, 'hardwareConcurrency', { get: () => 8, configurable: true });
        Object.defineProperty(navigator, 'deviceMemory', { get: () => 8, configurable: true });

        // 關鍵：修正視窗高度差（Pixelscan 檢測點）
        // 模擬標籤欄、地址欄與書籤欄的高度總和，通常在 80px - 100px 之間
        const decorationHeight = 85; 
        if (window.outerHeight <= window.innerHeight) {
            Object.defineProperty(window, 'outerHeight', {
                get: () => window.innerHeight + decorationHeight,
                configurable: true
            });
        }
    };

    // 4. 解決 Permissions 與 Plugins 異常
    const maskPermissions = () => {
        const originalQuery = window.navigator.permissions.query;
        window.navigator.permissions.query = (parameters) => (
            parameters.name === 'notifications' ?
                Promise.resolve({ state: Notification.permission }) :
                originalQuery(parameters)
        );
    };

    const maskPlugins = () => {
        // Pixelscan 會檢查插件長度，Headless 默認為 0。這裡偽裝成有 PDF 查看器。
        if (navigator.plugins.length === 0) {
            const mockPlugin = {
                0: { type: "application/pdf", suffixes: "pdf", description: "Portable Document Format", enabledPlugin: null },
                description: "Portable Document Format",
                filename: "internal-pdf-viewer",
                length: 1,
                name: "Chrome PDF Viewer"
            };
            Object.setPrototypeOf(mockPlugin, Plugin.prototype);
            
            Object.defineProperty(navigator, 'plugins', {
                get: () => [mockPlugin],
                configurable: true
            });
        }
    };

    // 5. 穿透型 Cloudflare Turnstile 智能點擊（保留原有功能）
    const cloudflareClicker = () => {
        const findAndClick = (root) => {
            const selectors = ['input[type=checkbox]', '#challenge-stage input', '.ctp-checkbox-label'];
            for (let s of selectors) {
                const node = root.querySelector(s);
                if (node && node.offsetWidth > 0 && node.offsetHeight > 0) {
                    node.click();
                    return true;
                }
            }
            const allElements = root.querySelectorAll('*');
            for (let el of allElements) {
                if (el.shadowRoot && findAndClick(el.shadowRoot)) return true;
            }
            return false;
        };

        setInterval(() => {
            try {
                if (!findAndClick(document)) {
                    document.querySelectorAll('iframe').forEach(ifr => {
                        try { if (ifr.contentDocument) findAndClick(ifr.contentDocument); } catch (e) {}
                    });
                }
            } catch (e) {}
        }, 800);
    };

    // 執行所有抹除任務
    maskWebDriver();
    maskChrome();
    maskHardware();
    maskPermissions();
    maskPlugins();
    cloudflareClicker();

    // 移除 Playwright 可能注入的檢測屬性
    try {
        delete window.__playwright;
        delete window.__pw_click;
    } catch (e) {}
})();