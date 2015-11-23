#
# Copyright (c) 2015 The heketi Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

import unittest
from heketi import *

TEST_ADMIN_KEY = "adminkey"
TEST_SERVER="http://localhost:8081"

class test_heketi(unittest.TestCase):

    def test_cluster(self):
        c = Client(TEST_SERVER,"admin",TEST_ADMIN_KEY)

        cluster, err = Cluster(c).create()
        self.assertEqual(True, err == '')
        self.assertEqual(True, cluster['id'] != "")
        self.assertEqual(True, len(cluster['nodes']) == 0)
        self.assertEqual(True, len(cluster['volumes']) == 0)

        # Request bad id
        info, err = Cluster(c).info("bad")
        self.assertEqual(True, err != '')
        self.assertEqual(True, info == '')

        # Get information about the client
        info, err = Cluster(c).info(cluster['id'])
        self.assertEqual(True, err == '')
        self.assertEqual(True, info == cluster)

        # Get a list of clusters
        list, err = Cluster(c).list()
        self.assertEqual(True, err == '')
        self.assertEqual(True, len(list['clusters']) == 1)
        self.assertEqual(True, list['clusters'][0] == cluster['id'])

        # Delete non-existent cluster
        err = Cluster(c).delete("badid")
        self.assertEqual(True, err != '')

        # Delete current cluster
        err = Cluster(c).delete(info['id'])
        self.assertEqual(True, err == '')


    def test_node(self):

        node_req = {}

        c = Client(TEST_SERVER,"admin",TEST_ADMIN_KEY)
        self.assertEqual(True, c != '')

        # Create cluster
        cluster, err = Cluster(c).create()
        self.assertEqual(True, err == '')
        self.assertEqual(True, cluster['id'] != "")
        self.assertEqual(True, len(cluster['nodes']) == 0)
        self.assertEqual(True, len(cluster['volumes']) == 0)

        # Add node to unknown cluster
        node_req['cluster'] = "bad_id"
        node_req['zone'] = 10
        node_req['hostnames'] = {
            "manage": [ "node1-manage.gluster.lab.com" ],
            "storage": [ "node1-storage.gluster.lab.com" ]
        }

        node, err = Node(c).add(**node_req)
        self.assertEqual(True, err != '')

        # Create node request packet
        node_req['cluster'] = cluster['id']
        node, err = Node(c).add(**node_req)
        self.assertEqual(True, err == '')
        self.assertEqual(True, node['zone'] == node_req['zone'])
        self.assertEqual(True, node['id'] != "")
        self.assertEqual(True, node_req['hostnames'] ==  node['hostnames'])
        self.assertEqual(True, len(node['devices']) == 0)


        # Info on invalid id
        info, err = Node(c).info("badid")
        self.assertEqual(True, err != '')
        self.assertEqual(True, info == '')

        # Get node info
        info, err = Node(c).info(node['id'])
        self.assertEqual(True, err == '')
        self.assertEqual(True, info == node)

        # Delete invalid node
        err = Node(c).delete("badid")
        self.assertEqual(True, err != '')

        # Can't delete cluster with a node
        err = Cluster(c).delete(cluster['id'])
        self.assertEqual(True, err != '')

        # Delete node
        err = Node(c).delete(node['id'])
        self.assertEqual(True, err == '')

        # Delete cluster
        err = Cluster(c).delete(cluster['id'])
        self.assertEqual(True, err == '')


    def test_device(self):
        #db := tests.Tempfile()
        #defer os.Remove(db)

        # Create app
        c = Client(TEST_SERVER,"admin",TEST_ADMIN_KEY)

        # Create cluster
        cluster, err = Cluster(c).create()
        self.assertEqual(True, err == '')

        # Create node
        node_req = {}
        node_req['cluster'] = cluster['id']
        node_req['zone'] = 10
        node_req['hostnames'] = {
            "manage" : [ "node1-manage.gluster.lab.com" ],
            "storage" : [ "node1-storage.gluster.lab.com" ]
        }

        node, err = Node(c).add(**node_req)
        self.assertEqual(True, err == '')

        # Create a device request
        device_req = {}
        device_req['name'] = "sda"
        device_req['weight'] = 100
        device_req['node'] = node['id']

        device, err = Device(c).add(**device_req)
        self.assertEqual(True, err == '')

        # Get node information
        info, err = Node(c).info(node['id'])
        self.assertEqual(True, err == '')
        self.assertEqual(True, len(info['devices']) == 1)
        self.assertEqual(True, len(info['devices'][0]['bricks']) == 0)
        self.assertEqual(True, info['devices'][0]['name'] == device_req['name'])
        self.assertEqual(True, info['devices'][0]['weight'] == device_req['weight'])
        self.assertEqual(True, info['devices'][0]['id'] != '')

        # Get info from an unknown id
        info_, err = Device(c).info("badid")
        self.assertEqual(True, err != '')

        # Get device information
        device_info, err = Device(c).info(info['devices'][0]['id'])
        self.assertEqual(True, err == '')
        self.assertEqual(True, device_info == info['devices'][0])

        # Try to delete node, and will not until we delete the device
        err = Node(c).delete(node['id'])
        self.assertEqual(True, err != '')

        # Delete unknown device
        err = Device(c).delete("badid")
        self.assertEqual(True, err != '')

        # Delete device
        err = Device(c).delete(device_info['id'])
        self.assertEqual(True, err == '')

        # Delete node
        err = Node(c).delete(node['id'])
        self.assertEqual(True, err == '')

        # Delete cluster
        err = Cluster(c).delete(cluster['id'])
        self.assertEqual(True, err == '')


    def test_volume(self):

        # Create cluster
        c = Client(TEST_SERVER,"admin",TEST_ADMIN_KEY)
        self.assertEqual(True, c != '')

        cluster, err = Cluster(c).create()
        self.assertEqual(True, err == '')

        # Create node request packet
        for i in range(4):
            node_req = {}
            node_req['cluster'] = cluster['id']
            node_req['hostnames'] = {
                "manage" : [ "node%s-manage.gluster.lab.com" %(i) ],
                "storage" : [ "node%s-storage.gluster.lab.com" %(i) ] }
            node_req['zone'] = i + 1

            # Create node
            node, err = Node(c).add(**node_req)
            self.assertEqual(True, err == '')

            # Create and add devices
            for i in range(1,20):
                device_req = {}
                device_req['name'] = "sda%s" %(i)
                device_req['weight'] = 100
                device_req['node'] = node['id']

                device, err = Device(c).add(**device_req)
                self.assertEqual(True, err == '')


        # Get list of volumes
        list, err = Volume(c).list()
        self.assertEqual(True, err == '')
        self.assertEqual(True, len(list['volumes']) == 0)

        # Create a volume
        volume_req = {}
        volume_req['size'] = 10
        volume, err = Volume(c).create(**volume_req)
        self.assertEqual(True, err == '')
        self.assertEqual(True, volume['id'] != "")
        self.assertEqual(True, volume['size'] == volume_req['size'])

        # Get list of volumes
        list, err = Volume(c).list()
        self.assertEqual(True, err == '')
        self.assertEqual(True, len(list['volumes']) == 1)
        self.assertEqual(True, list['volumes'][0] == volume['id'])

        # Get info on incorrect id
        info, err = Volume(c).info("badid")
        self.assertEqual(True, err != '')

        # Get info
        info, err = Volume(c).info(volume['id'])
        self.assertEqual(True, err == '')
        self.assertEqual(True, info == volume)

        # Expand volume with a bad id
        expand_size = 10
        volumeInfo, err = Volume(c).expand("badid", expand_size)
        self.assertEqual(True, err != '')

        # Expand volume
        volumeInfo, err = Volume(c).expand(volume['id'], expand_size)
        self.assertEqual(True, err == '')
        self.assertEqual(True, volumeInfo['size'] == 20)

        # Delete bad id
        err = Volume(c).delete("badid")
        self.assertEqual(True, err != '')

        # Delete volume
        err = Volume(c).delete(volume['id'])
        self.assertEqual(True, err == '')

        clusterInfo, err = Cluster(c).info(cluster['id'])
        for node_id in clusterInfo['nodes']:
            #Get node information
            nodeInfo, err = Node(c).info(node_id)
            self.assertEqual(True, err == '')

            # Delete all devices
            for device in nodeInfo['devices']:
                err = Device(c).delete(device['id'])
                self.assertEqual(True, err == '')

            #Delete node
            err = Node(c).delete(node_id)
            self.assertEqual(True, err == '')

        # Delete cluster
        err = Cluster(c).delete(cluster['id'])
        self.assertEqual(True, err == '')


if __name__ == '__main__':
    unittest.main()
