package ztest

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func RunShell(dir, bindir, script string, stdin io.Reader, useenvs, extraenvs []string) (string, string, error) {
	// "-e -o pipefile" ensures a test will fail if any command
	// fails unexpectedly.
	cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", script)
	cmd.Dir = dir
	cmd.Env = []string{
		"AppData=" + dir,      // For os.UserConfigDir on Windows.
		"HOME=" + dir,         // For os.User*Dir on Unix.
		"LocalAppData=" + dir, // For os.UserCacheDir on Windows.
		"PATH=" + bindir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"USERPROFILE=" + dir, // For os.UserHomeDir on Windows.
	}
	// Forward TMPDIR, TMP, and TEMP for os.TempDir.
	for _, env := range append(useenvs, "TMPDIR", "TMP", "TEMP") {
		if v, ok := os.LookupEnv(env); ok {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env, v))
		}
	}
	cmd.Env = append(cmd.Env, extraenvs...)
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
