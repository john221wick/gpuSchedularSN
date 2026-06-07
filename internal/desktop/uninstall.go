package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// UninstallResult reports what an uninstall did.
type UninstallResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// UninstallDesktopApp removes the installed desktop app, its launcher/icons and
// saved config, then quits. Mirrors what install.sh laid down.
func (a *App) UninstallDesktopApp() (UninstallResult, error) {
	switch runtime.GOOS {
	case "linux":
		if err := uninstallLinux(); err != nil {
			return UninstallResult{}, err
		}
	case "darwin":
		if err := uninstallDarwin(); err != nil {
			return UninstallResult{}, err
		}
	default:
		return UninstallResult{Status: "unsupported", Message: "Uninstall is not supported on this OS."}, nil
	}

	// Remove saved config/user data (~/.gpusched).
	if home, err := os.UserHomeDir(); err == nil {
		_ = os.RemoveAll(filepath.Join(home, ".gpusched"))
	}

	a.quitAfter(700 * time.Millisecond)
	return UninstallResult{Status: "uninstalled", Message: "Uninstalled. Closing app."}, nil
}

func uninstallLinux() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	share := filepath.Join(home, ".local", "share")
	_ = os.Remove(filepath.Join(home, ".local", "bin", desktopBinaryName))
	_ = os.Remove(filepath.Join(share, "applications", desktopEntryName))
	// Remove the icon from every hicolor size we installed.
	for _, sz := range []string{"16x16", "24x24", "32x32", "48x48", "64x64", "128x128", "256x256", "512x512"} {
		_ = os.Remove(filepath.Join(share, "icons", "hicolor", sz, "apps", desktopIconName+".png"))
	}
	if p, err := exec.LookPath("gtk-update-icon-cache"); err == nil {
		_ = exec.Command(p, "-f", "-t", filepath.Join(share, "icons", "hicolor")).Run()
	}
	if p, err := exec.LookPath("update-desktop-database"); err == nil {
		_ = exec.Command(p, filepath.Join(share, "applications")).Run()
	}
	return nil
}

func uninstallDarwin() error {
	exe, err := runningExe()
	if err != nil {
		return err
	}
	// exe = <bundle>.app/Contents/MacOS/gpusched-desktop
	bundle := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	if filepath.Ext(bundle) != ".app" {
		return fmt.Errorf("not running from an .app bundle: %s", bundle)
	}
	if err := os.RemoveAll(bundle); err != nil {
		return fmt.Errorf("remove %s: %w (try removing it manually)", bundle, err)
	}
	return nil
}

// quitAfter quits the Wails app after a short delay so the UI can show the
// result first.
func (a *App) quitAfter(d time.Duration) {
	if a.ctx == nil {
		return
	}
	go func() {
		time.Sleep(d)
		wailsRuntime.Quit(a.ctx)
	}()
}
