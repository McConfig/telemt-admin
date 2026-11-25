package totp

import (
	"testing"
	"time"
)

func FuzzCode(f *testing.F) {
	f.Add("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", int64(59))
	f.Add("JBSWY3DPEHPK3PXP", int64(1_700_000_000))
	f.Fuzz(func(t *testing.T, secret string, unix int64) {
		if len(secret) > 256 || unix < -1_000_000_000 || unix > 4_000_000_000 {
			return
		}
		at := time.Unix(unix, 0)
		code, err := Code(secret, at)
		if err != nil {
			return
		}
		if len(code) != 6 {
			t.Fatalf("код %q", code)
		}
		if _, _, err := Validate(secret, code, at, 1); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})
}
