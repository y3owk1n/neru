package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

const (
	dirLeft  = "left"
	dirRight = "right"
	dirUp    = "up"
	dirDown  = "down"
)

func TestParseDirection(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    domain.Direction
		wantErr bool
	}{
		{name: dirLeft, input: dirLeft, want: domain.DirectionLeft},
		{name: dirRight, input: dirRight, want: domain.DirectionRight},
		{name: dirUp, input: dirUp, want: domain.DirectionUp},
		{name: dirDown, input: dirDown, want: domain.DirectionDown},
		{name: "uppercase", input: "RIGHT", want: domain.DirectionRight},
		{name: "surrounding whitespace", input: "  " + dirDown + "  ", want: domain.DirectionDown},
		{name: "empty", input: "", wantErr: true},
		{name: "unknown", input: "sideways", wantErr: true},
		{name: "abbreviation is not accepted", input: "l", wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := domain.ParseDirection(testCase.input)

			if testCase.wantErr {
				require.Error(t, err)
				assert.Equal(t, derrors.CodeInvalidInput, derrors.GetCode(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestDirection_Delta(t *testing.T) {
	tests := []struct {
		direction      domain.Direction
		wantX          int
		wantY          int
		wantString     string
		wantRoundTrips bool
	}{
		{domain.DirectionLeft, -1, 0, dirLeft, true},
		{domain.DirectionRight, 1, 0, dirRight, true},
		// Y grows downward in shared coordinates, so up is negative.
		{domain.DirectionUp, 0, -1, dirUp, true},
		{domain.DirectionDown, 0, 1, dirDown, true},
		{domain.Direction(99), 0, 0, domain.UnknownDirection, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.wantString, func(t *testing.T) {
			deltaX, deltaY := testCase.direction.Delta()

			assert.Equal(t, testCase.wantX, deltaX)
			assert.Equal(t, testCase.wantY, deltaY)
			assert.Equal(t, testCase.wantString, testCase.direction.String())

			if !testCase.wantRoundTrips {
				return
			}

			parsed, err := domain.ParseDirection(testCase.direction.String())
			require.NoError(t, err)
			assert.Equal(
				t,
				testCase.direction,
				parsed,
				"String and ParseDirection should round-trip",
			)
		})
	}
}
