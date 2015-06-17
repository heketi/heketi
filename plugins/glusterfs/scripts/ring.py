#!/usr/bin/env python

from swift.common import ring,utils
import json
import os
import sys


# Check argument
if len(sys.argv) < 3:
    print "<brick num> <brick id>"
    sys.exit(1)

# Allows calls into Ring
utils.HASH_PATH_SUFFIX = 'endcap'
utils.HASH_PATH_PREFIX = ''

r = ring.Ring(os.getcwd(), 15, 'heketi')

output = {}
nodes = r.get_nodes('a', sys.argv[1], sys.argv[2])

for node in r.get_more_nodes(nodes[0]):
    nodes[1].append(node)

output['partition'] = nodes[0]
output['nodes'] = nodes[1]

print json.dumps(output)

