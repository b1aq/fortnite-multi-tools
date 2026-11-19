//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var ntdll = syscall.NewLazyDLL("ntdll.dll")

// queryTimer returns coarsest, finest and current timer resolution in 100 ns units.
func queryTimer() (uint32, uint32, uint32, bool) {
	var a, b, c uint32
	r, _, _ := ntdll.NewProc("NtQueryTimerResolution").Call(
		uintptr(unsafe.Pointer(&a)), uintptr(unsafe.Pointer(&b)), uintptr(unsafe.Pointer(&c)))
	return a, b, c, r == 0
}

// setTimer requests (on=true) or releases (on=false) a timer resolution for this process.
func setTimer(res uint32, on bool) (uint32, bool) {
	var cur uint32
	flag := uintptr(0)
	if on {
		flag = 1
	}
	r, _, _ := ntdll.NewProc("NtSetTimerResolution").Call(uintptr(res), flag, uintptr(unsafe.Pointer(&cur)))
	return cur, r == 0
}
