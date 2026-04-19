/* Chrome Stealth Final hardened version
   Integrated features: deep WebDriver removal, prototype cleanup, isTrusted patching,
   Shadow DOM click traversal, and window property masking.
*/
(() => {
    // 1. Core: deeply remove WebDriver and automation fingerprints.
    const maskWebDriver = () => {
        try {
            const proto = Navigator.prototype;

            // Remove webdriver from the prototype chain.
            delete proto.webdriver;
            
            Object.defineProperty(proto, 'webdriver', {
                get: () => undefined, 
                enumerable: false,
                configurable: false
            });

            // Mask window.outerHeight/Width to avoid headless detection.
            // In headless mode, outerHeight often matches innerHeight, which looks suspicious.
            if (window.outerHeight <= window.innerHeight) {
                const decorationHeight = 85; // Simulate the title bar and address bar height.
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

    // 2. Simulate real Chrome-specific objects.
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

    // 3. Deeply mask plugin and hardware-related prototypes.
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

    // 4. Event trust masking (patch isTrusted).
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

    // 5. Traversal-style Cloudflare Turnstile auto-clicker (supports Shadow DOM).
    const cloudflareClicker = () => {
        const findAndClick = (root) => {
            // Extended selector set.
            const selectors = [
                'input[type=checkbox]', 
                '#challenge-stage input', 
                '.ctp-checkbox-label',
                '#mark-as-read'
            ];
            
            for (let s of selectors) {
                const node = root.querySelector(s);
                // It must be visible.
                if (node && node.offsetWidth > 0 && node.offsetHeight > 0) {
                    node.click();
                    return true;
                }
            }

            // Recurse into Shadow Roots.
            const allElements = root.querySelectorAll('*');
            for (let el of allElements) {
                if (el.shadowRoot && findAndClick(el.shadowRoot)) {
                    return true;
                }
            }
            return false;
        };

        // Check every 500ms.
        setInterval(() => {
            if (!findAndClick(document)) {
                // Try same-origin iframes.
                document.querySelectorAll('iframe').forEach(ifr => {
                    try {
                        if (ifr.contentDocument) findAndClick(ifr.contentDocument);
                    } catch (e) {
                        // Ignore cross-origin iframes.
                    }
                });
            }
        }, 500);
    };

    // 6. Clean Playwright residue.
    const cleanup = () => {
        try {
            delete window.__playwright;
            delete window.__pw_cleanup;
            delete window.__PW_inspect;
        } catch (e) {}
    };

    // Apply the patch.
    maskWebDriver();
    maskChrome();
    maskHardware();
    maskEvents();
    cloudflareClicker();
    cleanup();
})();
