# -*- encoding: utf-8 -*-
#
# Fix orphan bricks in exported heketi database.
#
# Author: Patrice FERLET <metal3d@gmail.com>
# Licence: BSD

import json
import logging


def remove_brick(root, brick_id, brick):
    """ Remove a brick brick_id, referenced in device "dev_id"
    from the "root" dict.
    """

    dev_id = brick["Info"]["device"]
    volume_id = brick["Info"]["volume"]

    logging.info("brick %s dev %s volume %s" % (brick_id, dev_id, volume_id))

    devbricks = []
    volbricks = []
    if dev_id in root["deviceentries"]:
        devbricks = root["deviceentries"][dev_id]["Bricks"]
    if volume_id in root["volumeentries"]:
        logging.debug(
                "Volume %s found for brick %s" % (volume_id, brick_id))
        volbricks = root["volumeentries"][volume_id]["Bricks"]
    else:
        logging.debug(
                "Volume %s not found for brick %s" % (volume_id, brick_id))

    # find brick in devices, and remove it
    i = 0
    for brick in devbricks:
        if brick == brick_id:
            devbricks.pop(i)
        i += 1

    # find brick in volumes, and remove it
    i = 0
    for brick in volbricks:
        if brick == brick_id:
            volbricks.pop(i)
        i += 1

    del(root["brickentries"][brick_id])


def usage():
    print("""
Usage:
    fix.py input.json output.json
    """)


if __name__ == "__main__":
    import sys
    import argparse

    logging.getLogger().setLevel(logging.INFO)

    parser = argparse.ArgumentParser(description="Fix Heketi databaase")
    parser.add_argument(
        "--input",
        dest="inputfile",
        required=True,
        help="input json file")
    parser.add_argument(
        "--output",
        dest="outputfile",
        required=True,
        help="output file")

    args = parser.parse_args()

    try:
        db = json.load(open(args.inputfile, 'r'))
        out = open(args.outputfile, "w")
    except Exception as e:
        logging.exception(e)
        sys.exit(1)

    # find bricks that have empty path to remove them
    bricks = db["brickentries"]
    devices = db["deviceentries"]

    for brick_id, brick in bricks.items():
        p = brick["Info"]["path"]
        if p == "":
            remove_brick(db, brick_id, brick)

    # write new db as json
    json.dump(db, out)
