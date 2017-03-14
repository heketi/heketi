package xml2json

import (
	"bytes"
	"testing"

	sj "github.com/bitly/go-simplejson"
	"github.com/stretchr/testify/assert"
)

type bio struct {
	Firstname string
	Lastname  string
	Hobbies   []string
	Misc      map[string]string
}

// TestEncode ensures that encode outputs the expected JSON document.
func TestEncode(t *testing.T) {
	assert := assert.New(t)

	author := bio{
		Firstname: "Bastien",
		Lastname:  "Gysler",
		Hobbies:   []string{"DJ", "Running", "Tennis"},
		Misc: map[string]string{
			"Nationality": "Swiss",
			"City":        "Zürich",
			"foo":         "",
			"bar":         "\"quoted text\"",
			"esc":         "escaped \\ sanitized",
		},
	}

	// Build document
	root := &Node{}
	root.AddChild("firstname", &Node{
		Data: author.Firstname,
	})
	root.AddChild("lastname", &Node{
		Data: author.Lastname,
	})

	for _, h := range author.Hobbies {
		root.AddChild("hobbies", &Node{
			Data: h,
		})
	}

	misc := &Node{}
	for k, v := range author.Misc {
		misc.AddChild(k, &Node{
			Data: v,
		})
	}
	root.AddChild("misc", misc)

	// Convert to JSON string
	buf := new(bytes.Buffer)
	err := NewEncoder(buf).Encode(root)
	if err != nil {
		assert.NoError(err)
	}

	// Build SimpleJSON
	sj, err := sj.NewJson(buf.Bytes())
	res, err := sj.Map()
	assert.NoError(err)

	// Assertions
	assert.Equal(author.Firstname, res["firstname"])
	assert.Equal(author.Lastname, res["lastname"])

	resHobbies, err := sj.Get("hobbies").StringArray()
	assert.NoError(err)
	assert.Equal(author.Hobbies, resHobbies)

	resMisc, err := sj.Get("misc").Map()
	assert.NoError(err)
	for k, v := range resMisc {
		assert.Equal(author.Misc[k], v)
	}
}
