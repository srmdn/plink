package server

import (
	"net"
	"testing"
)

func TestOGLogoCannotReachInternalNetworks(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.2", "169.254.169.254", "100.64.0.1", "0.0.0.0", "::1", "::ffff:127.0.0.1", "fd00::1", "fe80::1", "64:ff9b::7f00:1", "2002:7f00:1::1"} {
		if publicLogoIP(net.ParseIP(value)) {
			t.Errorf("internal address accepted: %s", value)
		}
	}
	if !publicLogoIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public address rejected")
	}
}
