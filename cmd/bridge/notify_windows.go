//go:build windows

package main

func notifyReady() {} // no-op: Windows uses NSSM, not systemd
