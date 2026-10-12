//go:build windows

package i18n

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// systemLocale reads the user's locale name ("vi-VN"), since LANG is rarely
// set on Windows. Returned with an underscore so it matches like "vi_VN".
func systemLocale() string {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	buf := make([]uint16, 85) // LOCALE_NAME_MAX_LENGTH
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return strings.ReplaceAll(windows.UTF16ToString(buf), "-", "_")
}
