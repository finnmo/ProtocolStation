//go:build !windows

package main

import "github.com/coreos/go-systemd/v22/daemon"

func notifyReady() {
	daemon.SdNotify(false, daemon.SdNotifyReady)
}
