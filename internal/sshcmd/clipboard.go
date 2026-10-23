package sshcmd

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type clipboardTool struct {
	name string
	args []string
}

func clipboardTools(platform string) []clipboardTool {
	switch platform {
	case "darwin":
		return []clipboardTool{{name: "pbcopy"}}
	case "windows":
		// Explicit UTF-8 stdin avoids clip.exe's OEM-codepage corruption of CJK.
		return []clipboardTool{{"powershell.exe", []string{"-NoProfile", "-NonInteractive", "-Command", "[Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false); $text = [Console]::In.ReadToEnd(); Set-Clipboard -Value $text"}}}
	default:
		return []clipboardTool{{name: "wl-copy"}, {"xclip", []string{"-selection", "clipboard"}}, {"xsel", []string{"--clipboard", "--input"}}}
	}
}

func copyWith(text, platform string, run func(clipboardTool, string) error) error {
	var failures []string
	for _, tool := range clipboardTools(platform) {
		if err := run(tool, text); err == nil {
			return nil
		} else {
			failures = append(failures, tool.name+": "+err.Error())
		}
	}
	return fmt.Errorf("clipboard unavailable (%s)", strings.Join(failures, "; "))
}

func Copy(text string) error {
	return copyWith(text, runtime.GOOS, func(tool clipboardTool, text string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, tool.name, tool.args...)
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	})
}
