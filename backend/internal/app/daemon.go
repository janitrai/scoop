package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	daemonServeUnitName    = "scoop-serve.service"
	daemonFrontendUnitName = "scoop-frontend.service"
	systemdUnitDir         = "/etc/systemd/system"
)

var daemonUnitNames = []string{
	daemonServeUnitName,
	daemonFrontendUnitName,
}

func runDaemon(args []string) int {
	if len(args) == 0 {
		printDaemonUsage()
		return 2
	}

	action := strings.ToLower(strings.TrimSpace(args[0]))
	switch action {
	case "help", "-h", "--help":
		printDaemonUsage()
		return 0
	case "install":
		return runDaemonInstall(args[1:])
	case "uninstall":
		return runDaemonUninstall(args[1:])
	case "start":
		return runDaemonServiceAction("start", args[1:], true)
	case "stop":
		return runDaemonServiceAction("stop", args[1:], true)
	case "restart":
		return runDaemonServiceAction("restart", args[1:], true)
	case "status":
		return runDaemonServiceAction("status", args[1:], false)
	default:
		fmt.Fprintf(os.Stderr, "unknown daemon action: %s\n\n", args[0])
		printDaemonUsage()
		return 2
	}
}

func runDaemonInstall(args []string) int {
	fs := flag.NewFlagSet("daemon install", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	defaultUser := strings.TrimSpace(os.Getenv("USER"))
	if defaultUser == "" {
		defaultUser = "root"
	}

	userName := fs.String("user", defaultUser, "Run services as this Linux user")
	backendPort := fs.Int("backend-port", 8090, "Port for scoop-serve")
	frontendPort := fs.Int("frontend-port", 5173, "Port for scoop-frontend")
	scoopDir := fs.String("scoop-dir", "", "Scoop root containing backend/ and frontend/ (auto-detected if empty)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "daemon install does not accept positional args")
		return 2
	}
	if err := validatePort(*backendPort, "--backend-port"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := validatePort(*frontendPort, "--frontend-port"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if strings.TrimSpace(*userName) == "" {
		fmt.Fprintln(os.Stderr, "--user must not be empty")
		return 2
	}
	if err := requireRoot("install"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	resolvedScoopDir, err := resolveScoopDir(*scoopDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve --scoop-dir: %v\n", err)
		return 2
	}
	pnpmPath, nodePath, err := resolveNodeAndPnpm()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to locate node/pnpm in PATH: %v\n", err)
		return 1
	}

	serveUnit := buildServeUnitFile(strings.TrimSpace(*userName), resolvedScoopDir, *backendPort)
	frontendUnit := buildFrontendUnitFile(strings.TrimSpace(*userName), resolvedScoopDir, pnpmPath, nodePath, *frontendPort)

	if err := writeUnitFile(daemonServeUnitName, serveUnit); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write %s: %v\n", daemonServeUnitName, err)
		return 1
	}
	if err := writeUnitFile(daemonFrontendUnitName, frontendUnit); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write %s: %v\n", daemonFrontendUnitName, err)
		return 1
	}
	if err := runSystemctl("daemon-reload"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to reload systemd units: %v\n", err)
		return 1
	}

	enableArgs := append([]string{"enable"}, daemonUnitNames...)
	if err := runSystemctl(enableArgs...); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to enable services: %v\n", err)
		return 1
	}

	fmt.Printf("Installed %s and %s\n", daemonServeUnitName, daemonFrontendUnitName)
	fmt.Println("Services are enabled on boot. Run `scoop daemon start` to start them now.")
	return 0
}

func runDaemonUninstall(args []string) int {
	fs := flag.NewFlagSet("daemon uninstall", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "daemon uninstall does not accept positional args")
		return 2
	}
	if err := requireRoot("uninstall"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	stopArgs := append([]string{"stop"}, daemonUnitNames...)
	if err := runSystemctl(stopArgs...); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to stop one or more services: %v\n", err)
	}

	disableArgs := append([]string{"disable"}, daemonUnitNames...)
	if err := runSystemctl(disableArgs...); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to disable one or more services: %v\n", err)
	}

	for _, unitName := range daemonUnitNames {
		unitPath := filepath.Join(systemdUnitDir, unitName)
		if err := os.Remove(unitPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "Failed to remove %s: %v\n", unitPath, err)
			return 1
		}
	}

	if err := runSystemctl("daemon-reload"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to reload systemd units: %v\n", err)
		return 1
	}

	fmt.Printf("Removed %s and %s\n", daemonServeUnitName, daemonFrontendUnitName)
	return 0
}

func runDaemonServiceAction(action string, args []string, requireRootPrivileges bool) int {
	fs := flag.NewFlagSet("daemon "+action, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "daemon %s does not accept positional args\n", action)
		return 2
	}
	if requireRootPrivileges {
		if err := requireRoot(action); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	systemctlArgs := make([]string, 0, 3+len(daemonUnitNames))
	systemctlArgs = append(systemctlArgs, action)
	if action == "status" {
		systemctlArgs = append(systemctlArgs, "--no-pager")
	}
	systemctlArgs = append(systemctlArgs, daemonUnitNames...)

	if err := runSystemctl(systemctlArgs...); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to %s services: %v\n", action, err)
		return 1
	}
	return 0
}

func validatePort(port int, flagName string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535", flagName)
	}
	return nil
}

func requireRoot(action string) error {
	if os.Geteuid() == 0 {
		return nil
	}
	return fmt.Errorf("daemon %s requires root privileges; run with sudo: sudo scoop daemon %s", action, action)
}

func resolveScoopDir(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed != "" {
		absPath, err := filepath.Abs(trimmed)
		if err != nil {
			return "", fmt.Errorf("normalize path %q: %w", trimmed, err)
		}
		if !isScoopRoot(absPath) {
			return "", fmt.Errorf("%q must contain backend/ and frontend/ directories", absPath)
		}
		return absPath, nil
	}

	detected, err := autoDetectScoopDir()
	if err != nil {
		return "", err
	}
	if !isScoopRoot(detected) {
		return "", fmt.Errorf("auto-detected path %q does not contain backend/ and frontend/", detected)
	}
	return detected, nil
}

func autoDetectScoopDir() (string, error) {
	candidates := make([]string, 0, 6)

	if exePath, err := os.Executable(); err == nil {
		resolvedExePath := exePath
		if resolvedPath, err := filepath.EvalSymlinks(exePath); err == nil {
			resolvedExePath = resolvedPath
		}

		exeDir := filepath.Dir(resolvedExePath)
		candidates = append(candidates, exeDir, filepath.Dir(exeDir))
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd, filepath.Dir(cwd))
	}

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		absPath, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, exists := seen[absPath]; exists {
			continue
		}
		seen[absPath] = struct{}{}

		if isScoopRoot(absPath) {
			return absPath, nil
		}
	}

	return "", errors.New("unable to auto-detect scoop directory from executable location or cwd parent; use --scoop-dir")
}

func isScoopRoot(root string) bool {
	return isDir(filepath.Join(root, "backend")) && isDir(filepath.Join(root, "frontend"))
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func resolveNodeAndPnpm() (string, string, error) {
	pnpmPath, err := exec.LookPath("pnpm")
	if err != nil {
		return "", "", err
	}
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return "", "", err
	}

	return pnpmPath, nodePath, nil
}

func buildServeUnitFile(userName, scoopDir string, backendPort int) string {
	lines := []string{
		"[Unit]",
		"Description=Scoop backend API service",
		"After=network.target postgresql.service",
		"",
		"[Service]",
		"Type=simple",
		"User=" + userName,
		"WorkingDirectory=" + filepath.Join(scoopDir, "backend"),
		"ExecStart=/usr/local/bin/scoop serve --host 0.0.0.0 --port " + strconv.Itoa(backendPort),
		"Restart=on-failure",
		"RestartSec=5",
		"",
		"[Install]",
		"WantedBy=multi-user.target",
		"",
	}
	return strings.Join(lines, "\n")
}

func buildFrontendUnitFile(userName, scoopDir, pnpmPath, nodePath string, frontendPort int) string {
	lines := []string{
		"[Unit]",
		"Description=Scoop frontend dev server",
		"After=scoop-serve.service",
		"",
		"[Service]",
		"Type=simple",
		"User=" + userName,
		"WorkingDirectory=" + filepath.Join(scoopDir, "frontend"),
		fmt.Sprintf("Environment=\"PATH=%s\"", buildFrontendPathEnv(pnpmPath, nodePath)),
		fmt.Sprintf("ExecStart=%s run dev --host 0.0.0.0 --port %d --strictPort", pnpmPath, frontendPort),
		"Restart=on-failure",
		"RestartSec=5",
		"",
		"[Install]",
		"WantedBy=multi-user.target",
		"",
	}
	return strings.Join(lines, "\n")
}

func buildFrontendPathEnv(pnpmPath, nodePath string) string {
	pathParts := []string{
		filepath.Dir(pnpmPath),
		filepath.Dir(nodePath),
		"/usr/local/sbin",
		"/usr/local/bin",
		"/usr/sbin",
		"/usr/bin",
		"/sbin",
		"/bin",
	}

	deduped := make([]string, 0, len(pathParts))
	seen := map[string]struct{}{}
	for _, part := range pathParts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		deduped = append(deduped, part)
	}

	return strings.Join(deduped, ":")
}

func writeUnitFile(name, content string) error {
	unitPath := filepath.Join(systemdUnitDir, name)
	return os.WriteFile(unitPath, []byte(content), 0o644)
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func printDaemonUsage() {
	fmt.Fprintln(os.Stderr, "scoop daemon")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  scoop daemon <action> [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Actions:")
	fmt.Fprintln(os.Stderr, "  install     Write unit files, daemon-reload, and enable services on boot")
	fmt.Fprintln(os.Stderr, "  uninstall   Stop, disable, and remove unit files")
	fmt.Fprintln(os.Stderr, "  start       Start both services")
	fmt.Fprintln(os.Stderr, "  stop        Stop both services")
	fmt.Fprintln(os.Stderr, "  restart     Restart both services")
	fmt.Fprintln(os.Stderr, "  status      Show status for both services")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Install flags:")
	fmt.Fprintln(os.Stderr, "  --user <name>          Service user (default: $USER)")
	fmt.Fprintln(os.Stderr, "  --backend-port <n>     Backend port (default: 8090)")
	fmt.Fprintln(os.Stderr, "  --frontend-port <n>    Frontend port (default: 5173)")
	fmt.Fprintln(os.Stderr, "  --scoop-dir <path>     Scoop root directory (auto-detect by default)")
}
