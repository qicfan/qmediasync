//go:build windows
// +build windows

package database

import "syscall"

func setHideWindow(attr *syscall.SysProcAttr) {
	attr.HideWindow = true
}

func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW: 强制不创建控制台窗口
	}
}
