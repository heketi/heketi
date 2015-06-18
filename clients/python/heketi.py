#!/usr/bin/env python

import httplib
import json

conn = httplib.HTTPConnection('localhost:8080')
headers = { 'Content-type': 'application/json' }

conn.request('GET', '/nodes')
r = conn.getresponse()

print "Status ", r.status
print "packet"
print r.read()
