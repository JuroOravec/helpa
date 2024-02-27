package functions

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func TestIndentRestNoLines(t *testing.T) {
	assert := assert.New(t)

	result := IndentRest(4, "  Hello there!")
	assert.Equal("  Hello there!", result)
}

func TestIndentRestWithLines(t *testing.T) {
	assert := assert.New(t)

	result := IndentRest(4, "  Hello there!\nTraveller\n  What a nice day.")
	assert.Equal("  Hello there!\n    Traveller\n      What a nice day.", result)
}
