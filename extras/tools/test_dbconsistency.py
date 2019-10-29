#!/usr/bin/python
#
# Copyright (c) 2019 The heketi Authors
#
# This file is licensed to you under your choice of the GNU Lesser
# General Public License, version 3 or any later version (LGPLv3 or
# later), or the GNU General Public License, version 2 (GPLv2), in all
# cases as published by the Free Software Foundation.
#

import json
import subprocess
import unittest

J1 = """
{
    "clusterentries": {
        "7234de4476a10cb0d138e3fd3d387c40": {
            "Info": {
                "id": "7234de4476a10cb0d138e3fd3d387c40",
                "nodes": [
                    "0680dabe91ee5a7f36da8cb6fe49cdd4",
                    "648f2115e99bf41fb78271acd55bd8f9",
                    "b016e52ee7378debc04427385f81cd82"
                ],
                "volumes": [
                    "d5b2b04a138cc3ad981eea8af73c42e0"
                ],
                "block": true,
                "file": true,
                "blockvolumes": []
            }
        }
    },
    "volumeentries": {
        "d5b2b04a138cc3ad981eea8af73c42e0": {
            "Info": {
                "size": 10,
                "name": "vol_d5b2b04a138cc3ad981eea8af73c42e0",
                "durability": {
                    "type": "replicate",
                    "replicate": {
                        "replica": 3
                    },
                    "disperse": {}
                },
                "gid": 2017,
                "snapshot": {
                    "enable": false,
                    "factor": 1
                },
                "id": "d5b2b04a138cc3ad981eea8af73c42e0",
                "cluster": "7234de4476a10cb0d138e3fd3d387c40",
                "mount": {
                    "glusterfs": {
                        "hosts": [
                            "172.28.11.52",
                            "172.18.11.53",
                            "172.18.11.54"
                        ],
                        "device": "172.28.11.52:vol_d5b2b04a138cc3ad981eea8af73c42e0",
                        "options": {
                            "backup-volfile-servers": "172.18.11.53,172.18.11.54"
                        }
                    }
                },
                "blockinfo": {}
            },
            "Bricks": [
                "3d66e8af1f3c1734582dfa9f3a2e851b",
                "3fc5beedfd5ca3dedcb466cf03c1ac96",
                "78dd5ca3ea40c14db90ef0ae0514f00d"
            ],
            "GlusterVolumeOptions": [
                "server.tcp-user-timeout 42",
                ""
            ],
            "Pending": {
                "Id": ""
            }
        }
    },
    "brickentries": {
        "3d66e8af1f3c1734582dfa9f3a2e851b": {
            "Info": {
                "id": "3d66e8af1f3c1734582dfa9f3a2e851b",
                "path": "/var/lib/heketi/mounts/vg_de7e39a0914578585dacfd558d01ccda/brick_3d66e8af1f3c1734582dfa9f3a2e851b/brick",
                "device": "de7e39a0914578585dacfd558d01ccda",
                "node": "0680dabe91ee5a7f36da8cb6fe49cdd4",
                "volume": "d5b2b04a138cc3ad981eea8af73c42e0",
                "size": 10485760
            },
            "TpSize": 10485760,
            "PoolMetadataSize": 53248,
            "Pending": {
                "Id": ""
            },
            "LvmThinPool": "tp_13e8b74fe5ab447e1df9d2219ef665b3",
            "LvmLv": "",
            "SubType": 1
        },
        "3fc5beedfd5ca3dedcb466cf03c1ac96": {
            "Info": {
                "id": "3fc5beedfd5ca3dedcb466cf03c1ac96",
                "path": "/var/lib/heketi/mounts/vg_971a878445843f67fc0ef3426eb3bb6a/brick_3fc5beedfd5ca3dedcb466cf03c1ac96/brick",
                "device": "971a878445843f67fc0ef3426eb3bb6a",
                "node": "648f2115e99bf41fb78271acd55bd8f9",
                "volume": "d5b2b04a138cc3ad981eea8af73c42e0",
                "size": 10485760
            },
            "TpSize": 10485760,
            "PoolMetadataSize": 53248,
            "Pending": {
                "Id": ""
            },
            "LvmThinPool": "tp_3fc5beedfd5ca3dedcb466cf03c1ac96",
            "LvmLv": "",
            "SubType": 1
        },
        "78dd5ca3ea40c14db90ef0ae0514f00d": {
            "Info": {
                "id": "78dd5ca3ea40c14db90ef0ae0514f00d",
                "path": "/var/lib/heketi/mounts/vg_bc2782bf157d2cde474ff55ae298715f/brick_78dd5ca3ea40c14db90ef0ae0514f00d/brick",
                "device": "bc2782bf157d2cde474ff55ae298715f",
                "node": "b016e52ee7378debc04427385f81cd82",
                "volume": "d5b2b04a138cc3ad981eea8af73c42e0",
                "size": 10485760
            },
            "TpSize": 10485760,
            "PoolMetadataSize": 53248,
            "Pending": {
                "Id": ""
            },
            "LvmThinPool": "tp_78dd5ca3ea40c14db90ef0ae0514f00d",
            "LvmLv": "",
            "SubType": 1
        }
    },
    "nodeentries": {
        "648f2115e99bf41fb78271acd55bd8f9": {
            "State": "online",
            "Info": {
                "zone": 1,
                "hostnames": {
                    "manage": [
                        "two.example.com"
                    ],
                    "storage": [
                        "192.168.1.2"
                    ]
                },
                "cluster": "7234de4476a10cb0d138e3fd3d387c40",
                "id": "648f2115e99bf41fb78271acd55bd8f9"
            },
            "Devices": [
                "971a878445843f67fc0ef3426eb3bb6a"
            ]
        },
        "0680dabe91ee5a7f36da8cb6fe49cdd4": {
            "State": "online",
            "Info": {
                "zone": 1,
                "hostnames": {
                    "manage": [
                        "one.example.com"
                    ],
                    "storage": [
                        "192.168.1.1"
                    ]
                },
                "cluster": "7234de4476a10cb0d138e3fd3d387c40",
                "id": "0680dabe91ee5a7f36da8cb6fe49cdd4"
            },
            "Devices": [
                "de7e39a0914578585dacfd558d01ccda"
            ]
        },
        "b016e52ee7378debc04427385f81cd82": {
            "State": "online",
            "Info": {
                "zone": 1,
                "hostnames": {
                    "manage": [
                        "three.example.com"
                    ],
                    "storage": [
                        "192.168.1.3"
                    ]
                },
                "cluster": "7234de4476a10cb0d138e3fd3d387c40",
                "id": "b016e52ee7378debc04427385f81cd82"
            },
            "Devices": [
                "bc2782bf157d2cde474ff55ae298715f"
            ]
        }
    },
    "deviceentries": {
        "de7e39a0914578585dacfd558d01ccda": {
            "State": "online",
            "Info": {
                "name": "/dev/sdc",
                "storage": {
                    "total": 1048440832,
                    "free": 1037901824,
                    "used": 10539008
                },
                "id": "de7e39a0914578585dacfd558d01ccda"
            },
            "Bricks": [
                "3d66e8af1f3c1734582dfa9f3a2e851b"
            ],
            "NodeId": "0680dabe91ee5a7f36da8cb6fe49cdd4",
            "ExtentSize": 4096
        },
        "971a878445843f67fc0ef3426eb3bb6a": {
            "State": "online",
            "Info": {
                "name": "/dev/sdc",
                "storage": {
                    "total": 1048440832,
                    "free": 1037901824,
                    "used": 10539008
                },
                "id": "971a878445843f67fc0ef3426eb3bb6a"
            },
            "Bricks": [
                "3fc5beedfd5ca3dedcb466cf03c1ac96"
            ],
            "NodeId": "648f2115e99bf41fb78271acd55bd8f9",
            "ExtentSize": 4096
        },
        "bc2782bf157d2cde474ff55ae298715f": {
            "State": "online",
            "Info": {
                "name": "/dev/sdb",
                "storage": {
                    "total": 1048440832,
                    "free": 1037901824,
                    "used": 10539008
                },
                "id": "bc2782bf157d2cde474ff55ae298715f"
            },
            "Bricks": [
                "78dd5ca3ea40c14db90ef0ae0514f00d"
            ],
            "NodeId": "b016e52ee7378debc04427385f81cd82",
            "ExtentSize": 4096
        }
    },
    "blockvolumeentries": {},
    "dbattributeentries": {},
    "pendingoperations": {}
}
"""


class Report(object):
    def __init__(self, output):
        self.summary = {}
        self.issues = []
        for line in output.splitlines():
            if line.startswith('    ') and ':' in line:
                k, v = line.strip().split(':')
                self.summary[k] = int(v)
                continue
            if line.strip() == "":
                continue
            self.issues.append(line.strip())


class Result(object):
    def __init__(self, proc, out, err):
        self.returncode = proc.returncode
        self.output = out.decode('utf8')
        self.error = err.decode('utf8')
        self.report = Report(self.output)
        #self.debug()

    def debug(self):
        print ("Got: returncode={}".format(self.returncode))
        print ("---- out ----\n{}".format(self.output))
        print ("---- err ----\n{}".format(self.error))

    def isOk(self):
        return self.returncode == 0 and self.error == ""

    def hasError(self):
        return self.error != ""


def run_dbconsistency(json):
    python = 'python3'
    dbconsistency = 'extras/tools/dbconsistency.py'
    p = subprocess.Popen(
        [python, dbconsistency, '/dev/stdin'],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE)
    out, err = p.communicate(input=json.encode('utf8'))
    return Result(p, out, err)


class TestDbConsistencyTool(unittest.TestCase):
    def test_basic_json_consistent(self):
        r = run_dbconsistency(J1)
        self.assertTrue(r.isOk())
        self.assertEqual(r.report.summary['Clusters'], 1)
        self.assertEqual(r.report.summary['Volumes'], 1)
        self.assertEqual(r.report.summary['Bricks'], 3)
        self.assertEqual(r.report.summary['Devices'], 3)
        self.assertFalse(r.report.issues)

    def test_duplicate_id_cluster_volumes(self):
        o = json.loads(J1)
        c = o['clusterentries']['7234de4476a10cb0d138e3fd3d387c40']
        c['Info']['volumes'].append(c['Info']['volumes'][0])
        j = json.dumps(o)

        r = run_dbconsistency(j)
        self.assertFalse(r.isOk())
        self.assertFalse(r.hasError())
        self.assertTrue(r.report.issues)
        self.assertEqual(len(r.report.issues), 1)
        self.assertIn('7234de4476a10cb0d138e3fd3d387c40', r.report.issues[0])
        self.assertIn("duplicate ids in volume list", r.report.issues[0])

    def test_duplicate_id_device_bricks(self):
        o = json.loads(J1)
        d = o['deviceentries']['de7e39a0914578585dacfd558d01ccda']
        d['Bricks'].append(d['Bricks'][0])
        j = json.dumps(o)

        r = run_dbconsistency(j)
        self.assertFalse(r.isOk())
        self.assertFalse(r.hasError())
        self.assertTrue(r.report.issues)
        self.assertEqual(len(r.report.issues), 2)
        self.assertIn('de7e39a0914578585dacfd558d01ccda', r.report.issues[0])
        self.assertIn("duplicate ids in brick list", r.report.issues[0])

    def test_missing_brick(self):
        o = json.loads(J1)
        del o['brickentries']['78dd5ca3ea40c14db90ef0ae0514f00d']
        j = json.dumps(o)

        r = run_dbconsistency(j)
        self.assertFalse(r.isOk())
        self.assertFalse(r.hasError())
        self.assertTrue(r.report.issues)
        self.assertEqual(len(r.report.issues), 3)
        self.assertIn("78dd5ca3ea40c14db90ef0ae0514f00d", r.report.issues[0])
        self.assertIn("78dd5ca3ea40c14db90ef0ae0514f00d", r.report.issues[1])
        self.assertIn("unknown brick", r.report.issues[0])



if __name__ == '__main__':
    unittest.main()
