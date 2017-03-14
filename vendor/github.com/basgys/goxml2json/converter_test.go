package xml2json

import (
	"strings"
	"testing"

	sj "github.com/bitly/go-simplejson"
	"github.com/stretchr/testify/assert"
)

// TestConvert ensures that the whole process works correctly
// It takes an XML document and outputs a JSON document
func TestConvert(t *testing.T) {
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
	 <mixed attr="attribute">
	 	content
	 </mixed>
  </osm>`

	// Build SimpleJSON
	json, err := sj.NewJson([]byte(`{
	  "osm": {
	    "-version": "0.6",
	    "-generator": "CGImap 0.0.2",
	    "bounds": {
	      "-minlat": "54.0889580",
	      "-minlon": "12.2487570",
	      "-maxlat": "54.0913900",
	      "-maxlon": "12.2524800"
	    },
	    "node": [
	      {
	        "-id": "298884269",
	        "-lat": "54.0901746",
	        "-lon": "12.2482632",
	        "-user": "SvenHRO",
	        "-uid": "46882",
	        "-visible": "true",
	        "-version": "1",
	        "-changeset": "676636",
	        "-timestamp": "2008-09-21T21:37:45Z"
	      },
	      {
	        "-id": "261728686",
	        "-lat": "54.0906309",
	        "-lon": "12.2441924",
	        "-user": "PikoWinter",
	        "-uid": "36744",
	        "-visible": "true",
	        "-version": "1",
	        "-changeset": "323878",
	        "-timestamp": "2008-05-03T13:39:23Z"
	      },
	      {
	        "-id": "1831881213",
	        "-version": "1",
	        "-changeset": "12370172",
	        "-lat": "54.0900666",
	        "-lon": "12.2539381",
	        "-user": "lafkor",
	        "-uid": "75625",
	        "-visible": "true",
	        "-timestamp": "2012-07-20T09:43:19Z",
	        "tag": [
	          {
	            "-k": "name",
	            "-v": "Neu Broderstorf"
	          },
	          {
	            "-k": "traffic_sign",
	            "-v": "city_limit"
	          }
	        ]
	      }
	    ],
	    "foo": "bar",
			"mixed": {
				"-attr": "attribute",
				"#content": "content"
			}
	  }
	}`))
	assert.NoError(err)

	expected, err := json.MarshalJSON()
	assert.NoError(err)

	// Then encode it in JSON
	res, err := Convert(strings.NewReader(s))
	assert.NoError(err)

	// Assertion
	assert.JSONEq(string(expected), res.String(), "Drumroll")
}

func TestConvertWithNewLines(t *testing.T) {
	assert := assert.New(t)

	s := `<?xml version="1.0" encoding="UTF-8"?>
  <osm>
   <foo>
	 	foo

		bar
	</foo>
  </osm>`

	// Build SimpleJSON
	json, err := sj.NewJson([]byte(`{
	  "osm": {
	    "foo": "foo\n\n\t\tbar"
	  }
	}`))
	assert.NoError(err)

	expected, err := json.MarshalJSON()
	assert.NoError(err)

	// Then encode it in JSON
	res, err := Convert(strings.NewReader(s))
	assert.NoError(err)

	// Assertion
	assert.JSONEq(string(expected), res.String(), "Drumroll")
}

func TestConvertWithMixedTags(t *testing.T) {
	assert := assert.New(t)

	s := `<?xml version="1.0" encoding="UTF-8"?>
	<soap-env:Envelope xmlns:soap-env="http://schemas.xmlsoap.org/soap/envelope/">
	    <soap-env:Header>
	        <wsse:Security xmlns:wsse="http://schemas.xmlsoap.org/ws/2002/12/secext">
	            <wsse:BinarySecurityToken valueType="String" EncodingType="wsse:Base64Binary">
	                Shared/IDL:IceSess\/SessMgr:1\.0.IDL/Common/!ICESMS\/ACPCRTC!ICESMSLB\/CRT.LB!-3379045898978075261!1563026!0
	            </wsse:BinarySecurityToken>
	        </wsse:Security>
	    </soap-env:Header>
	</soap-env:Envelope> `

	// Build SimpleJSON
	json, err := sj.NewJson([]byte(`{
	  "Envelope": {
	    "Header": {
	      "Security": {
	        "-wsse": "http://schemas.xmlsoap.org/ws/2002/12/secext",
	        "BinarySecurityToken": {
	          "#content": "Shared/IDL:IceSess\\/SessMgr:1\\.0.IDL/Common/!ICESMS\\/ACPCRTC!ICESMSLB\\/CRT.LB!-3379045898978075261!1563026!0",
	          "-EncodingType": "wsse:Base64Binary",
	          "-valueType": "String"
	        }
	      }
	    },
	    "-soap-env": "http://schemas.xmlsoap.org/soap/envelope/"
	  }
	}`))
	assert.NoError(err)

	expected, err := json.MarshalJSON()
	assert.NoError(err)

	// Then encode it in JSON
	res, err := Convert(strings.NewReader(s))
	assert.NoError(err)

	// Assertion
	assert.JSONEq(string(expected), res.String(), "Drumroll")
}

// TestConvertISO ensures that other charsets can be converted
func TestConvertISO(t *testing.T) {
	assert := assert.New(t)

	s := []byte{0x3C, 0x3F, 0x78, 0x6D, 0x6C, 0x20, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6F, 0x6E, 0x3D, 0x22, 0x31, 0x2E, 0x30, 0x22, 0x20, 0x65, 0x6E, 0x63, 0x6F, 0x64, 0x69, 0x6E, 0x67, 0x3D, 0x22, 0x49, 0x53, 0x4F, 0x2D, 0x38, 0x38, 0x35, 0x39, 0x2D, 0x31, 0x22, 0x3F, 0x3E, 0x3C, 0x63, 0x68, 0x61, 0x72, 0x73, 0x65, 0x74, 0x3E, 0xFC, 0x62, 0x65, 0x72, 0x20, 0x63, 0x6F, 0x6D, 0x70, 0x6C, 0x65, 0x78, 0x3C, 0x2F, 0x63, 0x68, 0x61, 0x72, 0x73, 0x65, 0x74, 0x3E}

	// Build SimpleJSON
	json, err := sj.NewJson([]byte(`{
	  "charset": "über complex"
	}`))
	assert.NoError(err)

	expected, err := json.MarshalJSON()
	assert.NoError(err)

	// Then encode it in JSON
	res, err := Convert(strings.NewReader(string(s)))
	assert.NoError(err)

	// Assertion
	assert.JSONEq(string(expected), res.String(), "Drumroll")
}
