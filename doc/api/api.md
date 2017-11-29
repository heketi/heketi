# Contents
* [Overview](#overview)
* [Development](#development)
* [Authentication Model](#authentication-model)
* [Asynchronous Operations](#asynchronous-operations)
* [API](#api)

# Overview
Heketi provides a RESTful management interface which can be used to manage the life cycle of GlusterFS volumes.  The goal of Heketi is to provide a simple way to create, list, and delete GlusterFS volumes in multiple storage clusters.  Heketi intelligently will manage the allocation, creation, and deletion of bricks throughout the disks in the cluster.  Heketi first needs to learn about the topologies of the clusters before satisfying any requests.  It organizes data resources into the following: Clusters, contain Nodes, which contain Devices, which will contain Bricks.

# Development
To communicate with the Heketi service, you will need to either use a client library or directly communicate with the REST endpoints.  The following client libraries are supported: Go, Python

## Go Client Library
Here is a small example of how to use the Go client library:

```go

import (
	"fmt"
	"github.com/heketi/heketi/client/api/go-client"
)

func main() {
	// Create a client
	heketi := client.NewClient(options.Url, options.User, options.Key)

	// List clusters
	list, err := heketi.ClusterList()
	if err != nil {
		return err
	}

	output := strings.Join(list.Clusters, "\n")
	fmt.Fprintf(stdout, "Clusters:\n%v\n", output)
	return nil
}
```

For more examples see the [Heketi cli client](https://github.com/heketi/heketi/tree/master/client/cli/go/cmds).

* Source: https://github.com/heketi/heketi/tree/master/client/api/go-client

## Python Client Library
The python client library can be installed either from the source or by installing the `python-heketi` package in Fedora/RHEL.  The source is available https://github.com/heketi/heketi/tree/master/client/api/python , and for examples, please check out the [unit tests](https://github.com/heketi/heketi/blob/master/client/api/python/test/unit/test_client.py)

## Running the development server
The simplest way to development a client for Heketi is to run the Heketi service in `mock` mode.  In this mode, Heketi will not need to communicate with any storage nodes, instead it mocks the communication, while still supporting all REST calls and maintaining state.  The simplest way to run the Heketi server is to run it from a container as follows:

```
# docker run -d -p 8080:8080 heketi/heketi
# curl http://localhost:8080/hello
Hello from Heketi
```

# Authentication Model
Heketi uses a stateless authentication model based on the JSON Web Token (JWT) standard as proposed to the [IETF](https://tools.ietf.org/html/draft-ietf-oauth-json-web-token-25).  As specified by the specification, a JWT token has a set of _claims_ which can be added to a token to determine its correctness.  Heketi requires the use of the following standard claims:

* [_iss_](http://self-issued.info/docs/draft-ietf-oauth-json-web-token.html#rfc.section.4.1.1): Issuer.  Heketi supports two types of issuers:
    * _admin_: Has access to all APIs
    * _user_: Has access to only _Volume_ APIs     
* [_iat_](http://self-issued.info/docs/draft-ietf-oauth-json-web-token.html#rfc.section.4.1.6): Issued-at-time
* [_exp_](http://self-issued.info/docs/draft-ietf-oauth-json-web-token.html#rfc.section.4.1.4): Time when the token should expire

And a custom one following the model as described on [Atlassian](https://developer.atlassian.com/static/connect/docs/latest/concepts/understanding-jwt.html): 

* _qsh_.  URL Tampering prevention.

Heketi supports token signatures encrypted using the HMAC SHA-256 algorithm which is specified by the specification as `HS256`.

## Clients
There are JWT libraries available for most languages as highlighted on [jwt.io](http://jwt.io).  The client libraries allow you to easily create a JWT token which must be stored in the `Authorization: Bearer {token}` header.  A new token will need to be created for each REST call.  Here is an example of the header:

`Authorization: Bearer eyJhb[...omitted for brevity...]HgQ`

### Python Example
Here is an example of how to create a token as Python client:

```python
import jwt
import datetime
import hashlib

method = 'GET'
uri = '/volumes'
secret = 'My secret'

claims = {}

# Issuer
claims['iss'] = 'admin'

# Issued at time
claims['iat'] = datetime.datetime.utcnow()

# Expiration time
claims['exp'] = datetime.datetime.utcnow() \
	+ datetime.timedelta(minutes=10)

# URI tampering protection
claims['qsh'] = hashlib.sha256(method + '&' + uri).hexdigest()

print jwt.encode(claims, secret, algorithm='HS256')
```

Example output:

```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZG1pbiIsImlhdCI6MTQzNTY4MTY4OSwicXNoIjoiYzE2MmFjYzkwMjQyNzIxMjBiYWNmZmY3NzA5YzkzMmNjMjUyMzM3ZDBhMzBmYTE1YjAyNTAxMDA2NjY2MmJlYSIsImV4cCI6MTQzNTY4MjI4OX0.ZBd_NgzEoGckcnyY4_ypgJsN6Oi7x0KxX2w8AXVyiS8
```

### Ruby Example
Run this as: `./heketi-api.rb volumes`

```ruby
#!/usr/bin/env ruby

require 'jwt'
require 'digest'

user = "admin"
pass = "password"
server = "http://heketi.example.com:8443"

uri = "/#{ARGV[0]}"

payload = {}

headers = {
  iss: 'admin',
  iat: Time.now.to_i,
  exp: Time.now.to_i + 600,
  qsh: Digest::SHA256.hexdigest("GET&#{uri}")
}

token = JWT.encode headers, pass, 'HS256'

exec("curl -H \"Authorization: Bearer #{token}\" #{server}#{uri}")
```

Copy this example token and decode it in [jwt.io](http://jwt.io) by pasting it in the token area and changing the secret to `My secret`.

## More Information
* [JWT Specification](http://self-issued.info/docs/draft-ietf-oauth-json-web-token.html)
* [Debugger and clients at jwt.io](http://jwt.io)
* [Stateless tokens with JWT](http://jonatan.nilsson.is/stateless-tokens-with-jwt/)
* Clients
    * [Go JWT client](https://github.com/auth0/go-jwt-middleware)
    * [Python JWT client](https://github.com/jpadilla/pyjwt)
    * [Java JWT client](https://bitbucket.org/b_c/jose4j/wiki/Home)
    * [Ruby JWT client](https://github.com/jwt/ruby-jwt)


# Asynchronous Operations
Some operations may take a long time to process.  For these operations, Heketi will return [202 Accepted](http://httpstatus.es/202) with a temporary resource set inside the `Location` header.  A client can then issue a _GET_ on this temporary resource and receive the following:

* **HTTP Status 200**: Request is still in progress. _We may decide to add some JSON ETA data here in future releases_.
    * **Header** _X-Pending_ will be set to the value of _true_
* **HTTP Status 404**: Temporary resource requested is not found.
* **HTTP Status [500](http://httpstatus.es/500)**: Request completed and has failed.  Body will be filled in with error information.
* **HTTP Status [303 See Other](http://httpstatus.es/303)**: Request has been completed successfully. The information requested can be retrieved by issuing a _GET_ on the resource set inside the `Location` header.
* **HTTP Status [204 Done](http://httpstatus.es/204)**: Request has been completed successfully. There is no data to return.


# API

Details of the Heketi api are documented using OpenAPI (v 2.0) within the
[swagger.yml](./swagger.yml) in the doc/api dir of the Heketi repo.

For convenience we include an HTML page that displays the formatted
documentation (via swagger-ui) pointing at the
[lastest version of the API](./latest.html) that can be viewed directly
from the web. Please note that Heketi is expected to be run on private
clusters rather than the public Internet - thus any tools that attempt
to contact these APIs for examples are not expected to function.

Any version of the API documentation can be rendered/displayed by tools
that support OpenAPI 2.0 YAML content.
