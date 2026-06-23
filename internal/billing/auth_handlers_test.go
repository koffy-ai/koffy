package billing

import (
	"testing"

	"koffy/internal/config"
)

func TestSafeReturnTo(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "/center"},
		{name: "path", input: "/admin/apps?x=1", want: "/admin/apps?x=1"},
		{name: "external", input: "https://example.com", want: "/center"},
		{name: "allowed external", input: "https://app.example.com/work", want: "https://app.example.com/work"},
		{name: "protocol relative", input: "//example.com", want: "/center"},
		{name: "newline", input: "/admin\nSet-Cookie:x=y", want: "/center"},
	}

	server := &Server{cfg: config.Config{
		PublicWebURL: "https://koffy.example.com",
		AuthAllowedReturnOrigins: []string{
			"https://app.example.com",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := server.safeReturnTo(tt.input); got != tt.want {
				t.Fatalf("safeReturnTo(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
