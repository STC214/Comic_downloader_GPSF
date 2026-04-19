package browser

// BrowserSessionOptions controls how a browser middleware session is launched.
type BrowserSessionOptions struct {
	Headless         *bool          `json:"headless,omitempty"`
	ProfileDir       string         `json:"profileDir,omitempty"`
	UserAgent        string         `json:"userAgent,omitempty"`
	Locale           string         `json:"locale,omitempty"`
	TimezoneID       string         `json:"timezoneId,omitempty"`
	ViewportWidth    int            `json:"viewportWidth,omitempty"`
	ViewportHeight   int            `json:"viewportHeight,omitempty"`
	FirefoxUserPrefs map[string]any `json:"firefoxUserPrefs,omitempty"`
}

// HeadlessPtr returns a pointer to a bool for optional launch overrides.
func HeadlessPtr(value bool) *bool {
	return &value
}
