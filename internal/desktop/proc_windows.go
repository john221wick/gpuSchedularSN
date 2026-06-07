//go:build windows

package desktop

import "syscall"

// detachAttrs is a no-op placeholder on Windows (self-update is unsupported
// there; the desktop app is built only for macOS and Linux).
func detachAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}
