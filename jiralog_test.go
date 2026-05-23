package main

import (
	"context"
	"testing"
)

// TestReadCard tests the readCard() function with several inputs.
func TestReadCard(t *testing.T) {
	var tests = []struct {
		line         string
		prefix       string
		wantedPrefix string
		wantedOk     bool
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
		if got != test.wantedPrefix || ok != test.wantedOk {
			t.Errorf("readCard(%q, %q) = %s, %v | want %s, %v", test.line, test.prefix, got, ok, test.wantedPrefix, test.wantedOk)
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
		{"0910 test case 2A without colon", 910},
		{"1234: test case 2", 1234},
		{"1350: test case 2", 1350},
		{"0000: test case with all zero time", 0},
		{"1437", 1437},
		{"2359", 2359},
	}

	for _, test := range tests {
		got, err := readTime(test.input)
		if got != test.want || err != nil {
			t.Errorf("readTime(%q) = %d, %v, want %d", test.input, got, err, test.want)
		}
	}
}

// TestReadTimeWithInvalidInputs tests the readTime() function with several
// different invalid inputs.
func TestReadTimeWithInvalidInputs(t *testing.T) {
	var tests = []string{
		"1270: invalid minutes",
		"without time",
		"-124: negative time",
		"42", // short input
		"9959: invalid hours",
		"9900: invalid hours",
		"1160: invalid minutes",
		"2400: invalid hours",
	}

	for _, input := range tests {
		got, err := readTime(input)
		if err == nil {
			t.Errorf("readTime(%q) = %d, %v", input, got, err)
		}
	}
}

// TestGetWorklogSummaryErrors tests the errors from getWorklogSummary()
func TestGetWorklogSummaryErrors(t *testing.T) {
	var tests = []struct {
		key            string
		model          string
		prompt         string
		rawDescription string
		want           string
	}{
		{"", "model", "prompt", "description", "missing key"},
		{"key", "", "prompt", "description", "missing model"},
		{"key", "model", "", "description", "missing prompt"},
		{"key", "model", "prompt", "", "missing description"},
		{"", "", "", "", "missing key"},
	}

	for _, test := range tests {
		_, err := getWorklogSummary(context.TODO(), test.key, test.model, test.prompt, test.rawDescription)
		if err.Error() != test.want {
			t.Errorf("getWorklogSummary(%q, %q, %q, %q) = _, %q, wanted error %q", test.key, test.model, test.prompt, test.rawDescription, err, test.want)
		}
	}
}
