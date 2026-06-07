package desktop

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Version is the running app version, injected at build time via
//
//	-ldflags "-X github.com/john221wick/gpuSchedularSN/internal/desktop.Version=v0.1.11"
//
// Unset dev builds report "dev" and always treat an update as available so the
// flow can be exercised locally.
var Version = "dev"

// WebkitSuffix records which Linux webkit variant this binary was built for
// ("" for webkit2gtk-4.0, "-webkit41" for 4.1). Injected via -ldflags. The
// self-updater downloads the matching archive so the replacement keeps linking
// the webkit version already present on the machine. Empty on macOS.
var WebkitSuffix = ""

// desktopRepo hosts the desktop release archives (same repo install.sh uses).
const desktopRepo = "john221wick/gpuSchedularSN"

const desktopBinaryName = "gpusched-desktop"
const desktopAppBundle = "gpusched.app"
const desktopIconName = "gpusched-desktop"
const desktopEntryName = "gpusched-desktop.desktop"

// downloadClient allows long transfers (archives are tens of MB).
var downloadClient = &http.Client{Timeout: 10 * time.Minute}

// UpdateInfo is the result of an app update check.
type UpdateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	Notes     string `json:"notes"`
}

// GetAppVersion returns the running desktop app version.
func (a *App) GetAppVersion() string { return Version }

// CheckAppUpdate queries the latest desktop release and compares it to the
// running version.
func (a *App) CheckAppUpdate() (UpdateInfo, error) {
	tag, body, err := ghLatestRelease(desktopRepo)
	if err != nil {
		return UpdateInfo{Current: Version}, err
	}
	return UpdateInfo{
		Current:   Version,
		Latest:    tag,
		Available: updateAvailable(Version, tag),
		Notes:     strings.TrimSpace(body),
	}, nil
}

// DownloadAndApplyUpdate downloads the latest release archive for this platform
// (emitting "app-update-progress" 0..100), replaces the installed app, then
// relaunches. The caller (UI) should disable controls while this runs.
func (a *App) DownloadAndApplyUpdate() error {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return fmt.Errorf("self-update unsupported on %s", runtime.GOOS)
	}
	tag, _, err := ghLatestRelease(desktopRepo)
	if err != nil {
		return err
	}

	var asset string
	if runtime.GOOS == "linux" {
		asset = fmt.Sprintf("%s-linux-%s%s.tar.gz", desktopBinaryName, runtime.GOARCH, WebkitSuffix)
	} else {
		asset = fmt.Sprintf("%s-darwin-%s.tar.gz", desktopBinaryName, runtime.GOARCH)
	}
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", desktopRepo, tag, asset)

	tmp, err := os.MkdirTemp("", "gpusched-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	pkg := filepath.Join(tmp, "pkg.tar.gz")
	if err := a.downloadWithProgress(url, pkg, "app-update-progress"); err != nil {
		return err
	}
	extract := filepath.Join(tmp, "x")
	if err := extractTarGz(pkg, extract); err != nil {
		return fmt.Errorf("extract update: %w", err)
	}

	if runtime.GOOS == "darwin" {
		err = applyUpdateDarwin(extract)
	} else {
		err = applyUpdateLinux(extract)
	}
	if err != nil {
		return err
	}

	a.emit("app-update-progress", 100)
	a.relaunchAndQuit()
	return nil
}

// --- shared helpers --------------------------------------------------------

// ghLatestRelease returns the latest release tag and body for owner/repo.
func ghLatestRelease(repo string) (tag, body string, err error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	req.Header.Set("User-Agent", "gpusched-desktop") // GitHub rejects requests with no UA
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := downloadClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("check for updates: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("no release found for %s (HTTP %d)", repo, resp.StatusCode)
	}
	var rel struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", "", err
	}
	if rel.TagName == "" {
		return "", "", fmt.Errorf("no release found for %s", repo)
	}
	return rel.TagName, rel.Body, nil
}

// downloadWithProgress streams url to dest, emitting integer percentages on the
// given Wails event as it goes (and a final 100).
func (a *App) downloadWithProgress(url, dest, event string) error {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "gpusched-desktop")
	resp, err := downloadClient.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed (HTTP %d) — %s", resp.StatusCode, url)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	a.emit(event, 0)
	pr := &progressReader{r: resp.Body, total: resp.ContentLength, emit: func(pct int) { a.emit(event, pct) }}
	if _, err := io.Copy(out, pr); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	a.emit(event, 100)
	return nil
}

// progressReader wraps an io.Reader and reports download progress as a 0..100
// percentage, only emitting when the integer percent changes.
type progressReader struct {
	r     io.Reader
	total int64
	read  int64
	last  int
	emit  func(pct int)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if p.total > 0 && p.emit != nil {
		pct := int(p.read * 100 / p.total)
		if pct > 100 {
			pct = 100
		}
		if pct != p.last {
			p.last = pct
			p.emit(pct)
		}
	}
	return n, err
}

// extractTarGz unpacks a .tar.gz into destDir.
func extractTarGz(src, destDir string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		// Guard against path traversal in archive entries.
		target := filepath.Join(destDir, hdr.Name)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) && target != filepath.Clean(destDir) {
			return fmt.Errorf("unsafe path in archive: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			w, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return err
			}
			w.Close()
		}
	}
}

// updateAvailable reports whether latest is newer than current. A "dev" build
// always reports true so the update flow can be tested locally.
func updateAvailable(current, latest string) bool {
	if current == "dev" || current == "" {
		return latest != ""
	}
	return semverLess(current, latest)
}

// semverLess reports a < b for vX.Y.Z tags (missing/garbage parts sort low).
func semverLess(a, b string) bool {
	pa, pb := parseSemver(a), parseSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		if i > 2 {
			break
		}
		if j := strings.IndexAny(part, "-+"); j >= 0 {
			part = part[:j]
		}
		out[i], _ = strconv.Atoi(part)
	}
	return out
}

// --- platform apply + relaunch --------------------------------------------

// applyUpdateLinux replaces the installed binary and refreshes desktop
// integration (icons/.desktop). The new binary is renamed over the running one,
// which Linux permits (the running process keeps the old inode).
func applyUpdateLinux(extract string) error {
	exe, err := runningExe()
	if err != nil {
		return err
	}
	src := filepath.Join(extract, "bin", desktopBinaryName)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("update archive missing bin/%s", desktopBinaryName)
	}
	newPath := filepath.Join(filepath.Dir(exe), "."+desktopBinaryName+".new")
	if err := copyFile(src, newPath, 0o755); err != nil {
		return err
	}
	if err := os.Rename(newPath, exe); err != nil {
		os.Remove(newPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	// Best-effort desktop integration refresh (matches install.sh layout).
	home, _ := os.UserHomeDir()
	share := filepath.Join(home, ".local", "share")
	copyTree(filepath.Join(extract, "share", "icons"), filepath.Join(share, "icons"))
	if srcDesktop := filepath.Join(extract, "share", "applications", desktopEntryName); fileExists(srcDesktop) {
		iconPNG := filepath.Join(share, "icons", "hicolor", "256x256", "apps", desktopIconName+".png")
		_ = writeLinuxDesktopEntry(srcDesktop, filepath.Join(share, "applications", desktopEntryName), exe, iconPNG)
	}
	if p, err := exec.LookPath("gtk-update-icon-cache"); err == nil {
		_ = exec.Command(p, "-f", "-t", filepath.Join(share, "icons", "hicolor")).Run()
	}
	if p, err := exec.LookPath("update-desktop-database"); err == nil {
		_ = exec.Command(p, filepath.Join(share, "applications")).Run()
	}
	return nil
}

// writeLinuxDesktopEntry installs the .desktop, forcing Exec/TryExec to the
// absolute binary path and Icon to the absolute PNG so the menu launcher works
// regardless of session PATH or icon-cache state.
func writeLinuxDesktopEntry(srcDesktop, dstDesktop, exe, iconPNG string) error {
	data, err := os.ReadFile(srcDesktop)
	if err != nil {
		return err
	}
	var b strings.Builder
	hasTryExec := false
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "Exec="):
			line = "Exec=" + exe
		case strings.HasPrefix(line, "Icon="):
			line = "Icon=" + iconPNG
		case strings.HasPrefix(line, "TryExec="):
			line = "TryExec=" + exe
			hasTryExec = true
		}
		b.WriteString(line + "\n")
	}
	out := b.String()
	if !hasTryExec {
		out = strings.TrimRight(out, "\n") + "\nTryExec=" + exe + "\n"
	}
	if err := os.MkdirAll(filepath.Dir(dstDesktop), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dstDesktop, []byte(out), 0o644)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// applyUpdateDarwin swaps the running .app bundle for the freshly extracted one
// and clears the quarantine flag (the build is unsigned).
func applyUpdateDarwin(extract string) error {
	exe, err := runningExe()
	if err != nil {
		return err
	}
	// exe = <bundle>.app/Contents/MacOS/gpusched-desktop
	bundle := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	if !strings.HasSuffix(bundle, ".app") {
		return fmt.Errorf("not running from an .app bundle: %s", bundle)
	}
	src := filepath.Join(extract, desktopAppBundle)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("update archive missing %s", desktopAppBundle)
	}
	parent := filepath.Dir(bundle)
	staged := filepath.Join(parent, "."+desktopAppBundle+".new")
	old := filepath.Join(parent, "."+desktopAppBundle+".old")
	os.RemoveAll(staged)
	os.RemoveAll(old)
	if err := copyTree(src, staged); err != nil {
		return fmt.Errorf("stage new app: %w", err)
	}
	if err := os.Rename(bundle, old); err != nil {
		os.RemoveAll(staged)
		return fmt.Errorf("move old app: %w", err)
	}
	if err := os.Rename(staged, bundle); err != nil {
		os.Rename(old, bundle) // roll back
		return fmt.Errorf("install new app: %w", err)
	}
	os.RemoveAll(old)
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", bundle).Run()
	return nil
}

// relaunchAndQuit spawns a detached helper that waits for this process to exit
// then starts the updated app, and quits.
func (a *App) relaunchAndQuit() {
	exe, err := runningExe()
	if err != nil {
		return
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		bundle := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
		cmd = exec.Command("sh", "-c", fmt.Sprintf("sleep 1; open %q", bundle))
	} else {
		cmd = exec.Command("sh", "-c", fmt.Sprintf("sleep 1; %q", exe))
	}
	cmd.SysProcAttr = detachAttrs()
	_ = cmd.Start()
	if a.ctx != nil {
		wailsRuntime.Quit(a.ctx)
	}
}

// runningExe returns the resolved path of the running executable.
func runningExe() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

// emit sends a Wails event to the frontend (no-op before startup).
func (a *App) emit(event string, data ...interface{}) {
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, event, data...)
	}
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// copyTree recursively copies src into dst. Missing src is a no-op.
func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return nil // nothing to copy
	}
	if !info.IsDir() {
		return copyFile(src, dst, info.Mode())
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyTree(s, d); err != nil {
				return err
			}
			continue
		}
		fi, err := e.Info()
		if err != nil {
			return err
		}
		if err := copyFile(s, d, fi.Mode()); err != nil {
			return err
		}
	}
	return nil
}
