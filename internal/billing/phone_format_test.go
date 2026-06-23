package billing

import "testing"

func TestCasdoorMainlandPhoneFormat(t *testing.T) {
	localPhone := "138" + "0000" + "0000"
	e164Phone := "+86" + localPhone
	tests := []struct {
		name     string
		input    string
		casdoor  string
		internal string
	}{
		{name: "normalized", input: e164Phone, casdoor: localPhone, internal: e164Phone},
		{name: "local", input: localPhone, casdoor: localPhone, internal: e164Phone},
		{name: "with spaces", input: "+86 138 0000 0000", casdoor: localPhone, internal: e164Phone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := casdoorPhoneForMainland(tt.input); got != tt.casdoor {
				t.Fatalf("casdoorPhoneForMainland(%q) = %q, want %q", tt.input, got, tt.casdoor)
			}
			if got := internalPhoneFromCasdoorPhone(tt.input); got != tt.internal {
				t.Fatalf("internalPhoneFromCasdoorPhone(%q) = %q, want %q", tt.input, got, tt.internal)
			}
		})
	}
}

func TestPhoneLookupCandidates(t *testing.T) {
	localPhone := "138" + "0000" + "0000"
	e164Phone := "+86" + localPhone
	got := phoneLookupCandidates(e164Phone)
	want := []string{e164Phone, localPhone}
	if len(got) != len(want) {
		t.Fatalf("phoneLookupCandidates length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("phoneLookupCandidates[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
