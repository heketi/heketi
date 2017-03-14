package xml2json

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDecode ensures that decode does not return any errors (not that useful)
func TestDecode(t *testing.T) {
	assert := assert.New(t)

	s := `<?xml version="1.0" encoding="UTF-8"?>
  <osm version="0.6" generator="CGImap 0.0.2">
   <bounds minlat="54.0889580" minlon="12.2487570" maxlat="54.0913900" maxlon="12.2524800"/>
   <node id="298884269" lat="54.0901746" lon="12.2482632" user="SvenHRO" uid="46882" visible="true" version="1" changeset="676636" timestamp="2008-09-21T21:37:45Z"/>
   <node id="261728686" lat="54.0906309" lon="12.2441924" user="PikoWinter" uid="36744" visible="true" version="1" changeset="323878" timestamp="2008-05-03T13:39:23Z"/>
   <node id="1831881213" version="1" changeset="12370172" lat="54.0900666" lon="12.2539381" user="lafkor" uid="75625" visible="true" timestamp="2012-07-20T09:43:19Z">
    <tag k="name" v="Neu Broderstorf"/>
    <tag k="traffic_sign" v="city_limit"/>
   </node>
   <foo>bar</foo>
  </osm>`

	// Decode XML document
	root := &Node{}
	err := NewDecoder(strings.NewReader(s)).Decode(root)
	assert.NoError(err)
}

func TestTrim(t *testing.T) {
	table := []struct {
		in       string
		expected string
	}{
		{in: "foo", expected: "foo"},
		{in: " foo", expected: "foo"},
		{in: "foo ", expected: "foo"},
		{in: " foo ", expected: "foo"},
		{in: "   foo   ", expected: "foo"},
		{in: "foo bar", expected: "foo bar"},
		{in: "\n\tfoo\n\t", expected: "foo"},
		{in: "\n\tfoo\n\tbar\n\t", expected: "foo\n\tbar"},
		{in: "", expected: ""},
		{in: "\n", expected: ""},
		{in: "\n\v", expected: ""},
		{in: "ending with ä", expected: "ending with ä"},
		{in: "ä and ä", expected: "ä and ä"},
	}

	for _, scenario := range table {
		got := trimNonGraphic(scenario.in)
		assert.Equal(t, scenario.expected, got)
	}
}
