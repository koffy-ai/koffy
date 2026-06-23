package contracts

import "testing"

func TestCeilDiv(t *testing.T) {
	tests := []struct {
		name        string
		numerator   int64
		denominator int64
		want        int64
	}{
		{name: "exact", numerator: 1000, denominator: 1000, want: 1},
		{name: "round up", numerator: 1001, denominator: 1000, want: 2},
		{name: "zero", numerator: 0, denominator: 1000, want: 0},
		{name: "bad denominator", numerator: 1000, denominator: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CeilDiv(tt.numerator, tt.denominator); got != tt.want {
				t.Fatalf("CeilDiv(%d, %d) = %d, want %d", tt.numerator, tt.denominator, got, tt.want)
			}
		})
	}
}
