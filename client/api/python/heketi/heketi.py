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

import jwt
import datetime
import hashlib
import requests
import time
import json
import sys

class Client(object):

    def __init__(self, host, user, key):
        self.host = host
        self.user = user
        self.key = key


    def _set_token_in_header(self, headers, method, uri):
        claims = {}
        claims['iss'] = self.user

        # Issued at time
        claims['iat'] = datetime.datetime.utcnow()

        # Expiration time
        claims['exp'] = datetime.datetime.utcnow() \
                    + datetime.timedelta(seconds=1)

        # URI tampering protection
        claims['qsh'] = hashlib.sha256(method + '&' + uri).hexdigest()

        token = jwt.encode(claims, self.key, algorithm='HS256')
        headers['Authorization'] = 'bearer ' + token

    def hello(self):
        method = 'GET'
        uri = '/hello'

        headers={}
        self._set_token_in_header(headers, method, uri)
        r = requests.get(self.host + uri, headers=headers)
        return r.status_code == requests.codes.ok


class Cluster(object):
    """ Class to run cluster operations """

    def __init__(self, client):
        self.client = client

    def create(self):
        headers = {}
        method = 'POST'
        uri = "/clusters"

        output = ''
        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.post(self.client.host + uri, headers=headers)

        if r.status_code == requests.codes.created:
            output = r.json()
        else:
            err = r.status_code

        return output, err



    def info(self,cluster_id):
        headers = {}
        method = 'GET'
        uri = "/clusters/" + cluster_id

        output = ''
        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.get(self.client.host + uri, headers=headers)

        if r.status_code == requests.codes.ok:
            output = r.json()
        else:
            err = r.status_code

        return output, err



    def list(self):
        headers = {}
        method = 'GET'
        uri = "/clusters"

        output = ''
        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.get(self.client.host + uri, headers=headers)

        if r.status_code == requests.codes.ok:
            output = r.json()
        else:
            err = r.status_code

        return output, err



    def delete(self,cluster_id):
        """ Delete cluster. Returns only \
        error status code """

        headers = {}
        method = 'DELETE'
        uri = "/clusters/" + cluster_id

        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.delete(self.client.host + uri, headers=headers)

        if r.status_code != requests.codes.ok:
            err = r.status_code

        return err



class Node(object):

    def __init__(self, client):
        self.client = client

    def add(self, **kwargs):

        addnode_params = kwargs
        headers = {}
        method = 'POST'
        uri = "/nodes"
        queue_loc = ''

        output = ''
        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.post(self.client.host + uri,
                          data = json.dumps(addnode_params),
                          headers=headers)

        if r.status_code == requests.codes.accepted:
            queue_loc = r.headers['location']

            if queue_loc:
                headers = {}
                method = 'GET'
                uri = queue_loc

                response_ready = False
                node_uri = ''

                self.client._set_token_in_header(headers, method, uri)

                while response_ready is False:
                    q = requests.get(self.client.host + uri, headers=headers, allow_redirects=False)
                    if q.status_code == requests.codes.see_other:
                        node_uri = q.headers['location']
                        response_ready = True

                if node_uri:
                    headers = {}

                    self.client._set_token_in_header(headers, "GET", node_uri)
                    n = requests.get(self.client.host + node_uri, headers=headers)

                    if n.status_code == requests.codes.ok:
                        output = n.json()
                    else:
                        err = n.status_code
        else:
            err = r.status_code

        return output, err



    def info(self,node_id):
        self.node_id = node_id

        headers = {}
        method = 'GET'
        uri = "/nodes/" + self.node_id

        output = ''
        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.get(self.client.host + uri,
                          headers=headers)

        if r.status_code == requests.codes.ok:
            output = r.json()
        else:
            err = r.status_code

        return output, err



    def delete(self, node_id):
        self.node_id = node_id

        headers = {}
        method = 'DELETE'
        uri = "/nodes/"+ self.node_id

        queue_loc = ''
        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.delete(self.client.host + uri,
                          headers=headers)

        if r.status_code == requests.codes.accepted:
            queue_loc = r.headers['location']

            if queue_loc:
                headers = {}
                method = 'GET'
                uri = queue_loc

                response_ready = False

                self.client._set_token_in_header(headers, method, uri)
                while response_ready is False:
                    node_delete = requests.get(self.client.host + uri,
                                                 headers=headers,
                                                 allow_redirects=False)

                    if node_delete.status_code == requests.codes.NO_CONTENT:
                        response_ready = True
                    elif device_add.status_code == requests.codes.INTERNAL_SERVER_ERROR:
                        response_ready = True
                        err = node_delete.status_code

        else:
            err = r.status_code

        return err



class Device(object):

    def __init__(self, client):
        self.client = client

    def add(self, **kwargs):
        method = 'POST'
        uri = "/devices"
        headers={}

        output = ''
        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.post(self.client.host + uri,
                          data=json.dumps(kwargs),
                          headers=headers)

        if r.status_code == requests.codes.accepted:
            queue_loc = r.headers['location']

            if queue_loc:
                headers = {}
                method = 'GET'
                uri = queue_loc

                tmp_req = ''
                response_ready = False

                self.client._set_token_in_header(headers, method, uri)
                while response_ready is False:
                    device_add = requests.get(self.client.host + uri,
                                              headers=headers,
                                              allow_redirects=False)

                    if device_add.status_code == requests.codes.NO_CONTENT:
                        response_ready = True
                        output = device_add.status_code
                    elif device_add.status_code == requests.codes.INTERNAL_SERVER_ERROR:
                        response_ready = True
                        err = device_add.status_code

        else:
            err = r.status_code

        return output, err



    def info(self, device_id):
        self.device_id = device_id

        headers = {}
        method = 'GET'
        uri = "/devices/" + self.device_id

        output = ''
        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.get(self.client.host + uri, headers=headers)

        if r.status_code == requests.codes.ok:
            output = r.json()
        else:
            err = r.status_code

        return output, err



    def delete(self, device_id):
        self.device_id = device_id

        headers = {}
        method = 'DELETE'
        uri = "/devices/" + self.device_id
        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.delete(self.client.host + uri, headers=headers)

        if r.status_code == requests.codes.accepted:
            queue_loc = r.headers['location']

            if queue_loc:
                headers = {}
                method = 'GET'
                uri = queue_loc

                response_ready = False

                while response_ready is False:
                    self.client._set_token_in_header(headers, method, uri)
                    device_del = requests.get(self.client.host + uri,
                                           headers=headers,
                                           allow_redirects=False)
                    if device_del.status_code == requests.codes.no_content:
                        response_ready = True
        else:
            err = r.status_code

        return err



class Volume(object):

    def __init__(self, client):
        self.client = client

    def create(self, **kwargs):
        # TODO: checks for volume params
        vol_params = kwargs

        headers = {}
        method = 'POST'
        uri = "/volumes"

        output = ''
        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.post(self.client.host + uri,
                          data=json.dumps(vol_params),
                          headers=headers)

        if r.status_code == requests.codes.accepted:
            queue_loc = r.headers['location']

            if queue_loc:
                headers = {}
                method = 'GET'
                uri = queue_loc
                response_ready = False
                vol_req = ''

                while response_ready is False:
                    self.client._set_token_in_header(headers, method, uri)
                    tmp_req = requests.get(self.client.host + uri,
                                           headers=headers,
                                           allow_redirects=False)

                    if tmp_req.status_code == requests.codes.see_other:
                        response_ready = True
                        vol_req = tmp_req.headers['location']
                    elif tmp_req.status_code == requests.codes.INTERNAL_SERVER_ERROR:
                        # When volume creation fails
                        response_ready = True
                        err = tmp_req.status_code

                if vol_req:
                    headers = {}
                    method = 'GET'
                    uri = vol_req

                    self.client._set_token_in_header(headers, method, uri)
                    vol_info = requests.get(self.client.host + uri,
                                            headers=headers,
                                            allow_redirects=False)

                    if vol_info.status_code == requests.codes.ok:
                        output = vol_info.json()

        else:
            err = r.status_code

        return output, err



    def expand(self, volume_id, expand_size):

        self.volume_id = volume_id
        self.expand_size = expand_size

        vol_expand_params = dict({ 'expand_size' : self.expand_size })

        headers = {}
        method = 'POST'
        uri = "/volumes/" + self.volume_id + "/expand"

        output = ''
        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.post(self.client.host + uri,
                          data=json.dumps(vol_expand_params),
                          headers=headers)

        if r.status_code == requests.codes.accepted:
            queue_loc = r.headers['location']

            if queue_loc:
                headers = {}
                method = 'GET'
                uri = queue_loc
                vol_req = ''

                response_ready = False

                while response_ready is False:

                    self.client._set_token_in_header(headers, method, uri)
                    tmp_req = requests.get(self.client.host + uri,
                                           headers=headers,
                                           allow_redirects=False)

                    if tmp_req.status_code == requests.codes.see_other:
                        vol_req = tmp_req.headers['location']
                        response_ready = True
                    elif tmp_req.status_code == requests.codes.INTERNAL_SERVER_ERROR:
                        response_ready = True
                        err = tmp_req.status_code

                if vol_req:
                    headers = {}
                    method = 'GET'
                    uri = vol_req

                    self.client._set_token_in_header(headers, method, uri)
                    vol_info = requests.get(self.client.host + uri, headers=headers)

                    if vol_info.status_code == requests.codes.ok:
                        output = vol_info.json()
                    else:
                        err = vol_info.status_code
        else:
            err = r.status_code

        return output, err



    def info(self, volume_id):
        """ Get volume information """
        self.volume_id = volume_id

        headers = {}
        method = 'GET'
        uri = "/volumes/" + self.volume_id

        output = ''
        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.get(self.client.host + uri, headers=headers)

        if r.status_code == requests.codes.ok:
            output = r.json()
        else:
            err = r.status_code

        return output, err



    def list(self):
        """ List all volumes """

        headers = {}
        method = 'GET'
        uri = "/volumes"

        output = ''
        err = ''

        self.client._set_token_in_header(headers, method, uri)
        r = requests.get(self.client.host + uri, headers=headers)

        if r.status_code == requests.codes.ok:
            output = r.json()
        else:
            err = r.status_code

        return output, err



    def delete(self, volume_id):
        """ Delete a volume by passing \
            the volume id """

        self.volume_id = volume_id

        queue_loc = ''
        headers = {}
        method = 'DELETE'
        uri = "/volumes/" + volume_id

        err = ''

        self.client._set_token_in_header(headers,
                                         method, uri)
        r = requests.delete(self.client.host + uri,
                            headers=headers)


        if r.status_code == requests.codes.accepted:
            queue_loc = r.headers['location']

            if queue_loc:
                headers = {}
                method = 'GET'
                uri = queue_loc

                response_ready = False

                while response_ready is False:
                    self.client._set_token_in_header(headers, method, uri)
                    vol_del = requests.get(self.client.host + uri,
                                       headers=headers)

                    if vol_del.status_code == requests.codes.no_content:
                        response_ready = True
                    elif vol_del.status_code == requests.codes.NOT_FOUND:
                        err = vol_del.status_code
                        response_ready = True
        else:
            err = r.status_code

        return err
