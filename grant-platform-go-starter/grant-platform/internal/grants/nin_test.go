package grants

import "testing"

func TestNINFormat(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"01234567890", true}, {"12345678901", true},
		{"", false}, {"1234567890", false},
		{"123456789012", false}, {"1234567890a", false},
		{" 12345678901", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			s, in := fixture(t, 1)
			in.NIN = tc.value
			_, _, err := s.Create(in)
			if (err == nil) != tc.valid {
				t.Fatalf("NIN %q: valid=%v err=%v", tc.value, tc.valid, err)
			}
		})
	}
}
