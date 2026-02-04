//go:build windows
// +build windows

package helpers

import (
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/lxn/walk"
)

var WindowsExitChan = make(chan struct{})

func StartApp(stopFunc func()) {
	startWindow(stopFunc)
}

func startWindow(stopFunc func()) {
	// 创建隐藏的主窗口
	mainWindow, err := walk.NewMainWindow()
	if err != nil {
		log.Fatal(err)
	}

	// 设置托盘
	if err := setupFullFeaturedTray(mainWindow, stopFunc); err != nil {
		log.Fatal(err)
	}

	// 启动后台任务
	go func() {
		time.Sleep(5 * time.Second)
		openBrowser("http://127.0.0.1:12333")
	}()
	// // 运行应用
	mainWindow.Run()
}

var (
	notifyIcon *walk.NotifyIcon
)

func setupFullFeaturedTray(parent walk.Form, stopFunc func()) error {
	var err error
	// 加载图标
	icon, _ := walk.NewIconFromFile(filepath.Join(RootDir, "icon.ico"))

	// 创建托盘图标
	notifyIcon, err = walk.NewNotifyIcon(parent)
	if err != nil {
		return err
	}

	// 设置托盘属性
	if err = notifyIcon.SetIcon(icon); err != nil {
		return err
	}
	if err = notifyIcon.SetToolTip("QMediaSync正在后台运行中"); err != nil {
		return err
	}

	// 退出菜单项
	exitAction := walk.NewAction()
	exitAction.SetText("退出程序")
	exitAction.Triggered().Attach(func() {
		stopFunc()
		exitApp()
	})

	// We put an exit action into the context menu.
	exitAction.Triggered().Attach(func() { walk.App().Exit(0) })
	if err := notifyIcon.ContextMenu().Actions().Add(exitAction); err != nil {
		log.Fatal(err)
	}
	// 显示托盘图标
	notifyIcon.SetVisible(true)

	return nil
}

func exitApp() {
	notifyIcon.Dispose()
	walk.App().Exit(0)
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // linux, freebsd, etc.
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}
