#!/usr/bin/env python
#
# Copyright (c) 2018 The heketi Authors
#
# This file is licensed to you under your choice of the GNU Lesser
# General Public License, version 3 or any later version (LGPLv3 or
# later), or the GNU General Public License, version 2 (GPLv2), in all
# cases as published by the Free Software Foundation.
#
"""Test cases to check if TLS has been enabled
"""

import contextlib
import os
import socket
import subprocess
import sys
import time
import unittest

import requests


class SetupError(Exception):
    pass


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


class TestTLS(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.heketi = HeketiServer().start()

    def test_tls_enabled(self):
        resp = requests.get("https://localhost:8080/hello", verify="heketi.crt")
        self.assertEqual(resp.status_code, 200)

    @classmethod
    def tearDownClass(cls):
        cls.heketi.stop()


if __name__ == "__main__":
    unittest.main()
