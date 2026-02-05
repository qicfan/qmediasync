//go:build windows
// +build windows

package helpers

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/lxn/walk"
	"golang.org/x/sys/windows"
)

var mainWindow *walk.MainWindow
var ExitChan chan struct{} = make(chan struct{})

var stopFunction func()

func StartApp(stopFunc func()) {
	stopFunction = stopFunc
	startWindow()
}

func StopApp() {
	if mainWindow != nil {
		ExitChan <- struct{}{} // 发送关闭信号
		exitApp()
		mainWindow.Dispose()
		mainWindow = nil
	}
}

func startWindow() {
	// 创建隐藏的主窗口
	var err error
	mainWindow, err = walk.NewMainWindow()
	if err != nil {
		log.Fatal(err)
	}

	// 设置托盘
	if err := setupFullFeaturedTray(mainWindow, StopApp); err != nil {
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

func StartNewProcess(exePath, updateDir string) bool {
	// 复制一个临时的exe文件，启动这个临时文件，更新完成后删除
	var cmd *exec.Cmd
	if updateDir != "" {
		cmd = exec.Command(exePath, "-update", updateDir)
	} else {
		cmd = exec.Command(exePath)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	if err := cmd.Start(); err != nil {
		AppLogger.Errorf("启动更新进程失败: %v", err)
		return false
	}
	return true
}

// 检查进程是否存活 - Windows 专用
func IsProcessAlive(pid int) (bool, error) {
	// 定义常量
	const (
		PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
		STILL_ACTIVE                      = 259
	)

	// 打开进程
	handle, err := windows.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// 如果进程不存在，OpenProcess 会返回错误
		if err == windows.ERROR_INVALID_PARAMETER {
			return false, nil
		}
		if err == windows.ERROR_ACCESS_DENIED {
			// 有权限问题，但进程可能存活
			return true, nil
		}
		return false, err
	}
	defer windows.CloseHandle(handle)

	// 获取退出码
	var exitCode uint32
	err = windows.GetExitCodeProcess(handle, &exitCode)
	if err != nil {
		return false, err
	}

	// 如果退出码是 STILL_ACTIVE (259)，表示进程还在运行
	return exitCode == STILL_ACTIVE, nil
}
