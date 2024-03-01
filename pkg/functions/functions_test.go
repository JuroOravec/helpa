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

func TestJsonToYaml(t *testing.T) {
	assert := assert.New(t)

	result, err := JsonToYaml(`{"Value": ["1", 2, null, {"some": "value"}]}`)
	assert.Nil(err)
	assert.Equal("Value:\n- \"1\"\n- 2\n- null\n- some: value\n", result)
}

func TestYamlToJson(t *testing.T) {
	assert := assert.New(t)

	result, err := YamlToJson("Value:\n  - \"1\"\n  - 2\n  - null\n  - some: \"value\"")
	assert.Nil(err)
	assert.Equal(`{"Value":["1",2,null,{"some":"value"}]}`, result)
}
