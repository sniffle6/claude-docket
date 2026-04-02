package dashboard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// isWindowAlive checks if the terminal launched for a feature is still running.
// The .cmd script writes its PID (via a small PowerShell one-liner) to a .pid file.
// We check if that PID is still an active process.
func isWindowAlive(projDir, featureID string) bool {
	pidPath := filepath.Join(projDir, ".docket", "launch", featureID+".pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false // no PID file → not alive
	}

	pid := string(data)
	// Trim any whitespace/newlines
	for len(pid) > 0 && (pid[len(pid)-1] == '\n' || pid[len(pid)-1] == '\r' || pid[len(pid)-1] == ' ') {
		pid = pid[:len(pid)-1]
	}
	if pid == "" {
		return false
	}

	// Check if process exists — tasklist exits 0 if found
	cmd := exec.Command("cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: fmt.Sprintf(`cmd /C tasklist /FI "PID eq %s" /NH 2>nul | findstr /C:"%s" >nul`, pid, pid),
	}
	return cmd.Run() == nil
}

// launchInTerminal opens a new terminal window running claude for the given feature.
func launchInTerminal(projDir, promptPath, featureTitle, featureID, launchDir string) error {
	// Write a .cmd launcher script. Writes the cmd.exe PID to a .pid file so
	// isWindowAlive can check if the terminal is still running.
	// PowerShell's $PID parent is the cmd.exe running this script.
	pidPath := filepath.Join(launchDir, featureID+".pid")
	cmdScript := fmt.Sprintf("@echo off\r\ntitle docket-%s\r\npowershell -NoProfile -Command \"(Get-CimInstance Win32_Process -Filter ('ProcessId='+$PID)).ParentProcessId | Out-File -Encoding ascii -NoNewline '%s'\"\r\ncd /d \"%s\"\r\nclaude --dangerously-skip-permissions --append-system-prompt-file \"%s\" \"Resume work on: %s (feature_id: %s). Check get_ready for current status.\"\r\ndel \"%s\" 2>nul\r\n",
		featureID, pidPath, projDir, promptPath, featureTitle, featureID, pidPath)
	cmdPath := filepath.Join(launchDir, featureID+".cmd")
	if err := os.WriteFile(cmdPath, []byte(cmdScript), 0644); err != nil {
		return fmt.Errorf("failed to write launch script: %w", err)
	}

	vars := TemplateVars{
		FeatureID:    featureID,
		FeatureTitle: featureTitle,
		ScriptPath:   cmdPath,
		ProjectDir:   projDir,
	}

	cfg := ReadLaunchConfig(projDir)

	var cmdLine string
	if cfg.Launch != "" {
		cmdLine = "cmd /C " + SubstituteTemplate(cfg.Launch, vars, "windows")
	} else if _, err := exec.LookPath("wt"); err == nil {
		// Default: Windows Terminal with named window (no start wrapper needed)
		tmpl := `wt -w docket-{{feature_id}} --title {{feature_title}} cmd /k {{script_path}}`
		cmdLine = "cmd /C " + SubstituteTemplate(tmpl, vars, "windows")
	} else {
		// Fallback: no wt — use start to open in a new window
		tmpl := `cmd /c start {{feature_title}} cmd /k {{script_path}}`
		cmdLine = SubstituteTemplate(tmpl, vars, "windows")
	}

	cmd := exec.Command("cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: cmdLine,
	}
	cmd.Dir = projDir
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch: %w", err)
	}
	go cmd.Wait()
	return nil
}

// focusTerminal brings an existing terminal window for the given feature into focus.
func focusTerminal(projDir, featureID, featureTitle string) error {
	cfg := ReadLaunchConfig(projDir)
	if cfg.Focus == "" {
		return fmt.Errorf("no focus command configured in launch.toml")
	}

	vars := TemplateVars{
		FeatureID:    featureID,
		FeatureTitle: featureTitle,
		ProjectDir:   projDir,
	}

	cmdLine := "cmd /C " + SubstituteTemplate(cfg.Focus, vars, "windows")
	cmd := exec.Command("cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: cmdLine,
	}
	cmd.Dir = projDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("focus command failed: %w", err)
	}
	return nil
}
