package main

import "testing"

func TestLoopbackListen(t *testing.T) {
	tests := map[string]bool{
		"127.0.0.1:8080": true,
		"localhost:8080": true,
		"[::1]:8080":     true,
		"0.0.0.0:8080":   false,
		":8080":          false,
		"[::]:8080":      false,
		"example:8080":   false,
		"bad":            false,
	}
	for addr, want := range tests {
		if got := loopbackListen(addr); got != want {
			t.Errorf("%q: got %v, want %v", addr, got, want)
		}
	}
}
