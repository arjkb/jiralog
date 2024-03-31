package main

import "testing"

// TestReadTimeWithValidInputs tests the readTime() function with several
// different valid inputs.
func TestReadTimeWithValidInputs(t *testing.T) {
	var tests = []struct {
		input string
		want  int
	}{
		{"0123: test case 1", 123},
		{"0900: test case 2", 900},
		{"1234: test case 2", 1234},
		{"1350: test case 2", 1350},
		{"0000: test case with all zero time", 0},
	}

	for _, test := range tests {
		got, err := readTime(test.input)
		if got != test.want || err != nil {
			t.Errorf("readTime(%q) = %d, %v, want %d", test.input, got, err, test.want)
		}
	}
}

// TestReadTimeWithInvalidInputs rests the readTime() function with several
// different invalid inputs.
func TestReadTimeWithInvalidInputs(t *testing.T) {
	var tests = []string{
		"1270: invlid minutes",
		"without time",
		"-124: negative time",
	}

	for _, input := range tests {
		got, err := readTime(input)
		if err == nil {
			t.Errorf("readTime(%q) = %d, %v", input, got, err)
		}
	}
}
