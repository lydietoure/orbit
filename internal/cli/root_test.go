package cli

import "testing"

func TestEnvBool_UnsetIsFalse(t *testing.T) {
	// Use a key we explicitly leave unset.
	if got := envBool("ORBIT_TEST_ENVBOOL_DEFINITELY_UNSET"); got != false {
		t.Errorf("envBool(unset) = %v, want false", got)
	}
}
