package handlers

import "testing"

func TestCalculateSaturation(t *testing.T) {
	tests := []struct {
		name     string
		cidr     string
		devices  int
		expected float64
	}{
		{
			name:     "/24 with 32 devices",
			cidr:     "192.168.1.0/24",
			devices:  32,
			expected: 12.6,
		},
		{
			name:     "/30 fully utilized",
			cidr:     "192.168.1.0/30",
			devices:  2,
			expected: 100.0,
		},
		{
			name:     "invalid CIDR",
			cidr:     "10.0.0.0",
			devices:  50,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSaturation(tt.cidr, tt.devices)
			if diff := result - tt.expected; diff > 0.2 || diff < -0.2 {
				t.Fatalf("expected %.2f, got %.2f", tt.expected, result)
			}
		})
	}
}
