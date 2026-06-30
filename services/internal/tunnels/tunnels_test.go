package tunnels

import (
	"strings"
	"testing"
)

func TestGenPublicURL(t *testing.T) {
	a := genPublicURL()
	b := genPublicURL()

	if !strings.HasPrefix(a, "https://") {
		t.Errorf("public url should be https: %s", a)
	}
	if !strings.HasSuffix(a, "."+tunnelBaseDomain) {
		t.Errorf("public url should end with base domain: %s", a)
	}
	if a == b {
		t.Error("public urls should be unique")
	}
}
