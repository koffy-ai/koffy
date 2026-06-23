package auth

import "testing"

func TestBearerToken(t *testing.T) {
	token, err := bearerToken("Bearer abc.def")
	if err != nil {
		t.Fatal(err)
	}
	if token != "abc.def" {
		t.Fatalf("token = %q", token)
	}
}

func TestBearerTokenRejectsMissingScheme(t *testing.T) {
	if _, err := bearerToken("abc.def"); err == nil {
		t.Fatal("expected error")
	}
}
