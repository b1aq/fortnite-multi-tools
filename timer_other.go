//go:build !windows

package main

func queryTimer() (uint32, uint32, uint32, bool) { return 0, 0, 0, false }

func setTimer(res uint32, on bool) (uint32, bool) { return 0, false }
