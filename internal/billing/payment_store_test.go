package billing

import "testing"

func TestNewRechargeOrderNoWechatPayCompatible(t *testing.T) {
	orderNo, err := newRechargeOrderNo()
	if err != nil {
		t.Fatalf("newRechargeOrderNo() error = %v", err)
	}
	if len(orderNo) > 32 {
		t.Fatalf("order number length = %d, want <= 32: %s", len(orderNo), orderNo)
	}
	for _, char := range orderNo {
		if char >= '0' && char <= '9' {
			continue
		}
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			continue
		}
		t.Fatalf("order number contains unsupported character %q: %s", char, orderNo)
	}
}
