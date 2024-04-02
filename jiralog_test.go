package main

import "testing"

// TestReadCard tests the readCard() function with several inputs.
func TestReadCard(t *testing.T) {
	var tests = []struct {
		line          string
		prefix        string
		wanted_prefix string
		wanted_ok     bool
	}{
		{"1100 #BLAH-10 Expressa nocent, non expressa non nocent", "BLAH", "BLAH-10", true},
		{"1101 #BLAH-20 Falsus in uno, falsus in omnibus", "BLAH", "BLAH-20", true},
		{"1103 #BLAH-30 Ars longa, vita brevis", "BLAH", "BLAH-30", true},
		{"1104 #BLAH-40 Heredis fletus sub persona risus est", "BLAH", "BLAH-40", true},
		{"1105 #BLAH-50 Bis dat, qui cito dat", "BLAH", "BLAH-50", true},
		{"1106 #BLAH-60 Nascuntur poetae, fiunt oratores", "BLAH", "BLAH-60", true},
		{"1000 #BLAH-70 Nascuntur poetae, fiunt oratores", "NOTBLAH", "", false},
		{"1001 Nascuntur poetae, fiunt oratores", "BLAH", "", false},
		{"1002 BLAH-101 Nascuntur poetae, fiunt oratores", "BLAH", "", false},
		{"1003 BLAH-102 Nascuntur #poetae, fiunt oratores", "BLAH", "", false},
		{"1004 #BLAH Nascuntur #poetae, fiunt oratores", "BLAH", "", false},
		{"1005 #BLAH Nascuntur #poetae, fiunt oratores", "BLAH", "", false},
		{"2000 FooBar#BLAH-103 Nascuntur #poetae, fiunt oratores", "BLAH", "BLAH-103", true},
	}

	for _, test := range tests {
		got, ok := readCard(test.line, test.prefix)
		if got != test.wanted_prefix || ok != test.wanted_ok {
			t.Errorf("readCard(%q, %q) = %s, %v | want %s, %v", test.line, test.prefix, got, ok, test.wanted_prefix, test.wanted_ok)
		}
	}
}

// TestComputeDuration tests computeDuration with several different inputs.
func TestComputeDuration(t *testing.T) {
	var tests = []struct {
		start int
		stop  int
		want  int
	}{
		{900, 1000, 60},
		{900, 930, 30},
		{1400, 1459, 59},
		{1402, 1459, 57},
		{1555, 1605, 10},
		{900, 1100, 120},
		{1000, 1130, 90},
		{1030, 1300, 150},
		{1030, 1245, 135},
		{1100, 1100, 0},
		{1500, 1505, 5},
		{1040, 930, -1},
		{1260, 1340, -1},
		{1670, 1780, -1},
		{1300, 1480, -1},
		{12345, 1400, -1},
		{1400, 15678, -1},
		{435536, 32432, -1},
		{-1300, 1410, -1},
		{1300, -1410, -1},
		{-1300, -1410, -1},
	}

	for _, test := range tests {
		got, err := computeDuration(test.start, test.stop)
		if test.want != -1 {
			// valid cases
			if got != test.want || err != nil {
				t.Errorf("computeDuration(%d, %d) = %d, %v, want %d", test.start, test.stop, got, err, test.want)
			}
		} else {
			// invalid cases
			if err == nil {
				t.Errorf("computeDuration(%d, %d) = %d, %v, want %d", test.start, test.stop, got, err, test.want)
			}
		}
	}
}

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
		{"1437", 1437},
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
