//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"
)

const (
	appUID = 10001
	appGID = 10001
)

func dropPrivileges() error {
	if os.Geteuid() != 0 {
		return nil
	}
	if err := syscall.Setgroups([]int{}); err != nil {
		return fmt.Errorf("clear supplementary groups: %w", err)
	}
	if err := syscall.Setgid(appGID); err != nil {
		return fmt.Errorf("set gid %d: %w", appGID, err)
	}
	if err := syscall.Setuid(appUID); err != nil {
		return fmt.Errorf("set uid %d: %w", appUID, err)
	}
	if os.Geteuid() == 0 {
		return fmt.Errorf("uid remained root after privilege drop")
	}
	return nil
}
