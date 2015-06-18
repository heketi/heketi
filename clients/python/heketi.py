#!/usr/bin/env python

import httplib
import json
import string

DRIVES=24
NODES=24

conn = httplib.HTTPConnection('localhost:8080')
headers = { 'Content-type': 'application/json' }

for node in range(0,NODES):
    # Register node
    #conn.request('POST', '/nodes')
    info={}
    info['name'] = '192.168.10.%d' % (100+node)
    info['zone'] = node%4
    print json.dumps(info)

    print "Adding %s node" % (info['name'])
    conn.request('POST', '/nodes', json.dumps(info), headers)
    r = conn.getresponse()
    resp = json.loads(r.read())

    drives = {}
    drives['devices'] = []
    drive_letter = list(string.ascii_lowercase)
    for i in range(0,DRIVES):
        drive = {}
        drive['name'] = '/dev/sd%s' % (drive_letter[1+i])
        drive['weight'] = 100
        drives['devices'].append(drive)

    conn.request('POST', '/nodes/%s/devices' % (resp['id']), json.dumps(drives), headers)
    r = conn.getresponse()
    d = r.read()

conn.request('GET', '/nodes')
r = conn.getresponse()
fp = open('nodes', "w")
fp.write(r.read())
fp.close()
conn.close()



