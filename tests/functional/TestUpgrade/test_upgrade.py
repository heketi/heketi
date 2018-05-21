#!/usr/bin/env python
#
# Copyright (c) 2018 The heketi Authors
#
# This file is licensed to you under your choice of the GNU Lesser
# General Public License, version 3 or any later version (LGPLv3 or
# later), or the GNU General Public License, version 2 (GPLv2), in all
# cases as published by the Free Software Foundation.
#
"""Test cases to check basic behaviors post-upgrade.

Test cases that check basic system behaviors by loading a version
of the db from a previous version of the server. There are two
sets of cases: one that performs only "read" actions and one that
both reads and writes to the db.

Note that the server may write to the db on the read tests, however
we assume the major objects and their IDs persist across upgrade.
"""

import contextlib
import os
import socket
import subprocess
import sys
import time
import unittest

import heketi


class SetupError(Exception):
    pass


def _unpackdb(source, dest):
    with open(dest, 'wb') as destfh:
        p1 = subprocess.Popen(['unxz', '-c'],
                              stdin=subprocess.PIPE,
                              stdout=destfh)
        p2 = subprocess.Popen(['base64', '-d', source], stdout=p1.stdin)
        p1.stdin.close()
        p1.communicate()
        p2.communicate()
        if p1.returncode != 0:
            raise SetupError('base64 failed')
        if p2.returncode != 0:
            raise SetupError('unxz failed')


class HeketiServer(object):
    def __init__(self):
        self.heketi_bin = os.environ.get('HEKETI_SERVER', './heketi-server')
        self.log_path = os.environ.get('HEKETI_LOG', 'heketi.log')
        self._proc = None
        self._log = None

    def start(self):
        self._log = open(self.log_path, 'wb')
        self._proc = subprocess.Popen(
            [self.heketi_bin, '--config=heketi.json'],
            stdin=subprocess.PIPE,
            stdout=self._log,
            stderr=self._log)
        self._proc.stdin.close()
        time.sleep(0.25)
        if self._proc.poll() is not None:
            self.dump_log()
            raise SetupError('Heketi server failed to start')
        if not self.wait_for_heketi():
            self.stop()
            raise SetupError('Timed out waiting for Heketi to bind to port')
        return self

    def dump_log(self):
        with open(self.log_path) as fh:
            for line in fh.readlines():
                sys.stderr.write("HEKETI-LOG: {}".format(line))

    def wait_for_heketi(self):
        for _ in range(0, 30):
            time.sleep(1)
            s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            with contextlib.closing(s):
                if s.connect_ex(('127.0.0.1', 8080)) == 0:
                    return True
        return False

    def stop(self):
        self._proc.terminate()
        time.sleep(0.1)
        self._proc.kill()
        self._log.close()


class UpgradeTestBase(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        if cls.SOURCE_DB:
            try:
                _unpackdb(cls.SOURCE_DB, 'heketi.db')
            except SetupError:
                raise unittest.SkipTest("unable to unpack db")
        try:
            cls.heketi = HeketiServer().start()
        except SetupError:
            raise unittest.SkipTest('Heketi server failed to start')

    @classmethod
    def tearDownClass(cls):
        cls.heketi.stop()

    def heketi_client(self):
        """Return a fully configured heketi client object ready for use
        with the current test server.
        """
        return heketi.HeketiClient('http://localhost:8080', 'foo', 'bar')


class TestPostUpgrade(UpgradeTestBase):
    """Load a 4.0 Heketi db and see that basic read-only operations work.
    """
    SOURCE_DB = "heketi_40.db.xz.b64"

    def test_cluster_present(self):
        hc = self.heketi_client()
        res = hc.cluster_list()
        self.assertIn('clusters', res)
        clist = res['clusters']
        self.assertIn('40ab5e4524715e3a3cc75459b6c59e7d', clist)

    def test_volumes_present(self):
        expected = [
            "2e2210684ea1f8bebe6110818ce4c1ba",
            "302220e71d185a8213e00d7d3c96d7d8",
            "4975d61088bd444cbec7b6dde4d15aa9",
            "8af6b539928f9e1f0865f8dc12b95579",
            "c9495b84fa005c68ba62006bd8413914",
        ]
        hc = self.heketi_client()
        res = hc.volume_list()
        self.assertIn('volumes', res)
        volumes = res['volumes']
        self.assertTrue(set(expected).issubset(volumes))

    def test_volume_expected(self):
        hc = self.heketi_client()
        res = hc.volume_info("2e2210684ea1f8bebe6110818ce4c1ba")
        self.assertIn("size", res)
        self.assertEqual(5, res["size"])
        self.assertIn("name", res)
        self.assertEqual("beep", res["name"])
        expected_bricks = [
            "156e3fab0b62f58c11c0ef4ecbf1daaf",
            "764192fac007ba881c1e0c4a036ba2c1",
            "fdebcc12662deddaad0392103e228c89",
        ]
        self.assertIn("bricks", res)
        bricks = sorted(b['id'] for b in res["bricks"])
        self.assertEqual(expected_bricks, bricks)

    def test_nodes_devices_present(self):
        nd_expected = {
            "16b958279107f721a79760c653d3ba87": [
                "17085bce1016057aaad41fd113a6195b",
                "59735cadecf5fee20edce288c22ceda1",
                "cac0b9a641f223b757d759df08db7055"
            ],
            "3fd8095d420152b09f0b14b836b35c73": [
                "0828ff24e299f876f2688d7d86416970",
                "73aedff9a0b64a5a92d05d82754909b8",
                "7a383bc6ed7ac4d4f97cd69e4e07f95e"
            ],
            "9044637f522a28d956df487308767b0e": [
                "0282e09eac14dbd5f0c52bb4b5b3efe3",
                "225cd4081901bbdcbcc32461923b05a2",
                "22dca339dd79fc198e8af3478c2fd711"
            ],
            "b6978f5f16b14eca163db28d2b13b447": [
                "437f78a2fbbbacc7fc3950935e0af5c4",
                "c8309393dfe8935695066831df9cdd80",
                "ffad01ebc35e076ee1cee0a5459f0305"
            ],
        }
        hc = self.heketi_client()
        for node, devices in nd_expected.items():
            res = hc.node_info(node)
            self.assertIn("devices", res)
            device_ids = sorted(d['id'] for d in res['devices'])
            self.assertEqual(devices, device_ids)
        return

    def test_cluster_flags(self):
        # Here we are looking to see if the heketi 4.0 db we are started
        # with got updated with the expected cluster flags.
        hc = self.heketi_client()
        res = hc.cluster_info("40ab5e4524715e3a3cc75459b6c59e7d")
        self.assertIn("block", res)
        self.assertTrue(res["block"])
        self.assertIn("file", res)
        self.assertTrue(res["file"])


class TestPostUpgradeModify(UpgradeTestBase):
    """Load a 4.0 Heketi db and see that basic modification operations work.
    """
    SOURCE_DB = "heketi_40.db.xz.b64"

    def test_can_create_volume(self):
        req = {
            "size": 7,
            "name": "brand_new",
            "durability": {
                "type": "replicate",
                "replicate": {
                    "replica": 3
                }
            }
        }
        hc = self.heketi_client()
        res = hc.volume_create(req)
        self.assertIn("id", res)
        new_id = res["id"]
        # verify that the new volume is present in the cluster info
        res = hc.cluster_info("40ab5e4524715e3a3cc75459b6c59e7d")
        self.assertIn("volumes", res)
        self.assertIn(new_id, res["volumes"])

    def test_can_delete_old_volume(self):
        victim = "c9495b84fa005c68ba62006bd8413914"
        hc = self.heketi_client()
        # verify that it is there
        res = hc.volume_list()
        self.assertIn(victim, res.get('volumes', []))
        # delete it
        res = hc.volume_delete(victim)
        self.assertTrue(res)
        # verify it is gone
        res = hc.volume_list()
        self.assertNotIn(victim, res.get('volumes', []))


if __name__ == '__main__':
    unittest.main()
