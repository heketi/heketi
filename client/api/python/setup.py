# Copyright (c) 2016 heketi authors
#
# This file is licensed to you under your choice of the GNU Lesser
# General Public License, version 3 or any later version (LGPLv3 or
# later), or the GNU General Public License, version 2 (GPLv2), in all
# cases as published by the Free Software Foundation.


from setuptools import setup, find_packages

setup(
    name='heketi',
    version='3.0.0',
    description='Python client library for Heketi',
    license='LGPLv3+ or GPLv2'
    author='Luis Pabon',
    author_email='lpabon@redhat.com',
    url='https://github.com/heketi/heketi/tree/master/client/api/python',
    packages=find_packages(exclude=['test', 'bin']),
    test_suite='nose.collector',
    install_requires=['pyjwt', 'requests'],
    classifiers=[
        'Development Status :: 5 - Production/Stable',
        'Intended Audience :: Information Technology',
        'Intended Audience :: System Administrators',
        'License :: OSI Approved :: GNU Lesser General Public License v3 or later (LGPLv3+)',
        'Operating System :: POSIX :: Linux',
        'Programming Language :: Python',
        'Programming Language :: Python :: 2.7',
        'Topic :: System :: Filesystems',
        'Topic :: System :: Distributed Computing',
    ],
)
