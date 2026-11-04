package server

import "testing"

func TestProductPriceParsingAndDisplay(t *testing.T) {
	for _, test := range []struct {
		input    string
		minor    int64
		currency string
		label    string
	}{
		{"0", 0, "IDR", "Rp0"},
		{"79000", 7900000, "IDR", "Rp79.000"},
		{"12,5", 1250, "IDR", "Rp12,50"},
		{"19.99", 1999, "USD", "USD 19.99"},
	} {
		value, err := parseProductPrice(test.input)
		if err != nil || value == nil || *value != test.minor || productPriceLabel(value, test.currency) != test.label {
			t.Errorf("price %q: %v, %v", test.input, value, err)
		}
	}
	if value, err := parseProductPrice(""); err != nil || value != nil {
		t.Fatal("empty price should remain unset")
	}
	for _, invalid := range []string{"-1", "1.234", "1e3", "Rp100", "9999999999999"} {
		if _, err := parseProductPrice(invalid); err == nil {
			t.Errorf("accepted invalid price %q", invalid)
		}
	}
}
