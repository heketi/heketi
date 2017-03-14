package xml2json

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddChild(t *testing.T) {
	assert := assert.New(t)

	n := Node{}
	assert.Len(n.Children, 0)

	n.AddChild("a", &Node{})
	assert.Len(n.Children, 1)

	n.AddChild("b", &Node{})
	assert.Len(n.Children, 2)
}

func TestIsComplex(t *testing.T) {
	assert := assert.New(t)

	n := Node{}
	assert.False(n.IsComplex(), "nodes with no children are not complex")

	n.AddChild("b", &Node{})
	assert.True(n.IsComplex(), "nodes with children are complex")

	n.Data = "foo"
	assert.True(n.IsComplex(), "data does not impact IsComplex")
}
