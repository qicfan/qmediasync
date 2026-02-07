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

var IsFirstRun bool = false // 默认为 false

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
	// 只有第一次启动才打开网页
	if IsFirstRun {
		go func() {
			// 第一次启动建议多等一会儿，因为数据库初始化需要时间
			time.Sleep(5 * time.Second)
			openBrowser("http://127.0.0.1:12333")
			// 启动完后重置，防止程序内部逻辑错误（可选）
			IsFirstRun = false
		}()
	}
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

	// 创建“打开控制面板”菜单项
	openWebAction := walk.NewAction()
	openWebAction.SetText("打开控制面板")
	openWebAction.Triggered().Attach(func() {
		// 这里直接调用你已经写好的 openBrowser 函数
		openBrowser("http://127.0.0.1:12333")
	})
	// 将该项加入右键菜单
	if err := notifyIcon.ContextMenu().Actions().Add(openWebAction); err != nil {
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
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// 使用 cmd /c start 虽然方便，但必须设置 HideWindow
		cmd = exec.Command("cmd", "/c", "start", url)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} // 关键：隐藏子进程窗口
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
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
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000, // 0x08000000 是 CREATE_NO_WINDOW
		HideWindow:    true,
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

// Command 是对 exec.Command 的包装，在 Windows 下会自动隐藏窗口
func Command(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)

	if runtime.GOOS == "windows" {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		// 核心：设置隐藏窗口和创建无窗口标志
		cmd.SysProcAttr.HideWindow = true
		cmd.SysProcAttr.CreationFlags = 0x08000000 // CREATE_NO_WINDOW
	}

	return cmd
}
