//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// enableANSI turns on ANSI colour support, UTF-8 output and sets the window title.
func enableANSI() {
	k := syscall.NewLazyDLL("kernel32.dll")
	getMode := k.NewProc("GetConsoleMode")
	setMode := k.NewProc("SetConsoleMode")

	var mode uint32
	r, _, _ := getMode.Call(uintptr(syscall.Stdout), uintptr(unsafe.Pointer(&mode)))
	if r != 0 {
		setMode.Call(uintptr(syscall.Stdout), uintptr(mode|0x0004))
	}
	k.NewProc("SetConsoleOutputCP").Call(65001)

	if title, err := syscall.UTF16PtrFromString("Fortnite Multi Tool"); err == nil {
		k.NewProc("SetConsoleTitleW").Call(uintptr(unsafe.Pointer(title)))
	}
}
