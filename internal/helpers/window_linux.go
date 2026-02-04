//go:build !windows
// +build !windows

package helpers

var WindowsExitChan = make(chan struct{})

func StartApp(stopFunc func()) {
}
