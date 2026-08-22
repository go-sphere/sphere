package secure

import "testing"

// TestCensorString pins the masking rules documented on the function: the first
// and last characters survive for recognition, everything in between becomes
// asterisks, and degenerate inputs degrade to asterisks instead of panicking.
func TestCensorString(t *testing.T) {
	tests := []struct {
		name string
		src  string
		len  int
		want string
	}{
		{name: "long string", src: "13812345678", len: 11, want: "1*********8"},
		{name: "longer output than input", src: "abcdef", len: 8, want: "a******f"},
		{name: "two characters", src: "ab", len: 4, want: "a**b"},
		{name: "single character repeats at both ends", src: "a", len: 4, want: "a**a"},
		{name: "empty source becomes asterisks", src: "", len: 4, want: "****"},
		{name: "short output", src: "abc", len: 1, want: "*"},
		{name: "zero output", src: "abc", len: 0, want: ""},
		{name: "multibyte runes are preserved", src: "密码", len: 4, want: "密**码"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CensorString(tt.src, tt.len); got != tt.want {
				t.Fatalf("CensorString(%q, %d) = %q, want %q", tt.src, tt.len, got, tt.want)
			}
		})
	}
}
