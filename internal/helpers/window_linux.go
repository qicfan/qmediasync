//go:build !windows
// +build !windows

package helpers

var ExitChan chan struct{} = make(chan struct{})
var IsFirstRun bool = false // 默认为 false

func StartApp(stopFunc func()) {
}

func StopApp() {}

func StartNewProcess(exePath, updateDir string) bool {
	return true
}

func IsProcessAlive(pid int) (bool, error) {
	return true, nil
}
