package mold

import (
	"strconv"
	"testing"
)

func TestPos(t *testing.T) {
	source := `<html>

<head>
    <title>Test</title>
</head>

<body>
    <p>Hello world</p>
</body>

</html>
`
	tests := []struct {
		body         string
		pos          int
		expectedLine int
		expectedCol  int
	}{
		{
			body:         "",
			pos:          0,
			expectedLine: 1,
			expectedCol:  1,
		},
		{
			body:         source,
			pos:          0,
			expectedLine: 1,
			expectedCol:  1,
		},
		{
			body:         source,
			pos:          5,
			expectedLine: 1,
			expectedCol:  6,
		},
		{
			body:         source,
			pos:          20,
			expectedLine: 4,
			expectedCol:  6,
		},
		{
			body:         source,
			pos:          30,
			expectedLine: 4,
			expectedCol:  16,
		},
		{
			body:         source,
			pos:          50,
			expectedLine: 7,
			expectedCol:  3,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			line, col := pos(tt.body, tt.pos)
			if line != tt.expectedLine || col != tt.expectedCol {
				t.Errorf("pos(%q, %d) = (%d, %d), expected (%d, %d)", tt.body, tt.pos, line, col, tt.expectedLine, tt.expectedCol)
			}
		})
	}
}
