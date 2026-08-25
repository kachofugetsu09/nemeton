package daemon

import "testing"

func TestListenLoopbackRejectsNonLoopbackAddress(t *testing.T) {
	for _, address := range []string{"0.0.0.0:7373", "localhost:7373", "example.invalid:7373"} {
		if listener, err := listenLoopback(address); err == nil {
			listener.Close()
			t.Fatalf("listenLoopback(%q) succeeded", address)
		}
	}
	listener, err := listenLoopback("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listenLoopback(loopback): %v", err)
	}
	listener.Close()
}
