package config

import (
	"strings"
	"testing"
)

func TestValidateSocketPathUsesPlatformByteLimit(t *testing.T) {
	withinLimit := "/" + strings.Repeat("a", maximumUnixSocketPathBytes-1)
	if err := validateSocketPath(withinLimit); err != nil {
		t.Fatalf("validate %d-byte Socket path: %v", len(withinLimit), err)
	}

	overLimit := withinLimit + "b"
	if err := validateSocketPath(overLimit); err == nil || !strings.Contains(err.Error(), "Unix Socket path exceeds") {
		t.Fatalf("over-limit Socket path error = %v", err)
	}
}
