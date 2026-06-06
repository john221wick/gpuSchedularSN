package desktop

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const installerURL = "https://raw.githubusercontent.com/john221wick/gpuSchedularSN/main/install.sh"

type UpdateResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (a *App) UpdateDesktopApp() (UpdateResult, error) {
	var err error

	switch runtime.GOOS {
	case "darwin":
		err = runMacOSUpdater()
	case "linux":
		err = runLinuxUpdater()
	default:
		return UpdateResult{Status: "unsupported", Message: "Desktop updates are not supported on this OS yet."}, nil
	}

	if err != nil {
		return UpdateResult{}, err
	}

	a.relaunchAfterUpdate()
	return UpdateResult{Status: "updated", Message: "Update installed. Restarting app."}, nil
}

func runMacOSUpdater() error {
	shellCommand := fmt.Sprintf(`/bin/bash -lc %s`, shellQuote(fmt.Sprintf(
		`tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT; curl -fsSL %s -o "$tmp/install.sh"; bash "$tmp/install.sh" --desktop`,
		installerURL,
	)))
	script := "do shell script " + appleScriptQuote(shellCommand) + " with administrator privileges"
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("update failed: %s", commandError(err, out))
	}
	return nil
}

func runLinuxUpdater() error {
	shellCommand := fmt.Sprintf("curl -fsSL %s | bash -s -- --desktop", shellQuote(installerURL))
	out, err := exec.Command("/bin/sh", "-lc", shellCommand).CombinedOutput()
	if err != nil {
		return fmt.Errorf("update failed: %s", commandError(err, out))
	}
	return nil
}

func (a *App) relaunchAfterUpdate() {
	if a.ctx == nil {
		return
	}
	go func() {
		time.Sleep(700 * time.Millisecond)
		switch runtime.GOOS {
		case "darwin":
			_ = exec.Command("/bin/sh", "-lc", "sleep 1; open /Applications/gpusched.app").Start()
		case "linux":
			_ = exec.Command("/bin/sh", "-lc", `sleep 1; nohup "$HOME/.local/bin/gpusched-desktop" >/dev/null 2>&1 &`).Start()
		}
		wailsRuntime.Quit(a.ctx)
	}()
}

func commandError(err error, out []byte) string {
	output := strings.TrimSpace(string(out))
	if output == "" {
		return err.Error()
	}
	return err.Error() + ": " + output
}

func appleScriptQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}
