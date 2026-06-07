//go:build !windows

package desktop

import "syscall"

// detachAttrs detaches the relaunch helper into its own session so it survives
// this process quitting.
func detachAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
