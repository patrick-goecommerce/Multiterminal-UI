//go:build windows

package main

import (
	"os/exec"
	"testing"
)

func TestHideChildWindowSetsNoWindowFlag(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "echo")
	hideChildWindow(cmd)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&0x08000000 == 0 {
		t.Fatalf("CREATE_NO_WINDOW (0x08000000) not set on SysProcAttr = %+v", cmd.SysProcAttr)
	}
}
