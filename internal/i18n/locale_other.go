//go:build !windows

package i18n

// systemLocale has nothing to add outside Windows: the locale variables are
// the system setting there.
func systemLocale() string { return "" }
