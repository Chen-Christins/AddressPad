// Package sshcmd builds argv for system OpenSSH without invoking a shell.
package sshcmd

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"addresspad/internal/model"
)

func Build(s model.Server) ([]string, error) {
	s.Normalize()
	if err := s.Validate(); err != nil {
		return nil, err
	}
	args := []string{}
	if s.Port != 22 {
		args = append(args, "-p", strconv.Itoa(s.Port))
	}
	if s.IdentityFile != "" {
		path, err := model.ExpandHome(s.IdentityFile)
		if err != nil {
			return nil, err
		}
		args = append(args, "-i", path)
	}
	if s.ProxyJump != "" {
		args = append(args, "-J", s.ProxyJump)
	}
	args = append(args, s.ExtraArgs...)
	destination := s.Host
	if s.User != "" {
		destination = s.User + "@" + destination
	}
	args = append(args, "--", destination)
	for _, arg := range args {
		if strings.ContainsAny(arg, "\x00\r\n") {
			return nil, fmt.Errorf("SSH arguments must not contain NUL or newlines")
		}
	}
	return args, nil
}

func Command(s model.Server) (*exec.Cmd, error) {
	args, err := Build(s)
	if err != nil {
		return nil, err
	}
	path, err := exec.LookPath("ssh")
	if err != nil {
		return nil, fmt.Errorf("OpenSSH not found; install ssh and add it to PATH: %w", err)
	}
	return exec.Command(path, args...), nil
}

// Format returns a POSIX shell command, or a PowerShell command on Windows.
// Every argument is quoted; this is display/copy only, never used to execute SSH.
func Format(args []string, platform string) string {
	parts := []string{"ssh"}
	if platform == "windows" {
		parts[0] = "& ssh"
	}
	for _, arg := range args {
		if platform == "windows" {
			parts = append(parts, "'"+strings.ReplaceAll(arg, "'", "''")+"'")
		} else {
			parts = append(parts, "'"+strings.ReplaceAll(arg, "'", "'\"'\"'")+"'")
		}
	}
	return strings.Join(parts, " ")
}

func Display(s model.Server) (string, error) {
	args, err := Build(s)
	if err != nil {
		return "", err
	}
	return Format(args, runtime.GOOS), nil
}

// ExitCode distinguishes successful exit, SSH exit status and launch failure.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return -1
}
