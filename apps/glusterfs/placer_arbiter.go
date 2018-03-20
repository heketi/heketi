//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"fmt"

	"github.com/heketi/heketi/pkg/utils"
)

var (
	tryPlaceAgain error = fmt.Errorf("Placement failed. Try again.")

	// default discount size of an arbiter brick
	ArbiterDiscountSize uint64 = 64 * KB
)

const (
	arbiter_index int = 2
)

// ArbiterBrickPlacer is a Brick Placer implementation that can
// place bricks for arbiter volumes. It works primarily by
// dividing the devices into two "pools" - one for data bricks
// and one for arbiter bricks and understanding that only
// the last brick in the brick set is an arbiter brick.
type ArbiterBrickPlacer struct {
	// the following two function vars are to better support
	// dep. injection & unit testing
	canHostArbiter func(*DeviceEntry) bool
	canHostData    func(*DeviceEntry) bool
}

// Arbiter opts supports passing arbiter specific options
// across layers in the arbiter code along with the
// original placement opts.
type arbiterOpts struct {
	o         PlacementOpts
	brickSize uint64
}

func newArbiterOpts(opts PlacementOpts) *arbiterOpts {
	bsize, _ := opts.BrickSizes()
	return &arbiterOpts{
		o:         opts,
		brickSize: bsize,
	}
}

func (aopts *arbiterOpts) discount(index int) (err error) {
	if index == arbiter_index {
		aopts.brickSize, err = discountBrickSize(
			aopts.brickSize, ArbiterDiscountSize)
	}
	return
}

// NewArbiterBrickPlacer returns a new placer for bricks in
// a volume that supports the arbiter feature.
func NewArbiterBrickPlacer() *ArbiterBrickPlacer {
	return &ArbiterBrickPlacer{
		canHostArbiter: func(d *DeviceEntry) bool { return true },
		canHostData:    func(d *DeviceEntry) bool { return true },
	}
}

// PlaceAll constructs a full BrickAllocation for a volume that
// supports the arbiter feature.
func (bp *ArbiterBrickPlacer) PlaceAll(
	dsrc DeviceSource,
	opts PlacementOpts,
	pred DeviceFilter) (
	*BrickAllocation, error) {

	r := &BrickAllocation{
		BrickSets:  []*BrickSet{},
		DeviceSets: []*DeviceSet{},
	}

	numBrickSets := opts.SetCount()
	for sn := 0; sn < numBrickSets; sn++ {
		logger.Info("Allocating brick set #%v", sn)
		bs, ds, err := bp.newSets(
			dsrc,
			opts,
			pred)
		if err != nil {
			return r, err
		}
		r.BrickSets = append(r.BrickSets, bs)
		r.DeviceSets = append(r.DeviceSets, ds)
	}

	return r, nil
}

// Replace swaps out a brick & device in the input brick set at the
// given index for a new brick.
func (bp *ArbiterBrickPlacer) Replace(
	dsrc DeviceSource,
	opts PlacementOpts,
	pred DeviceFilter,
	bs *BrickSet,
	index int) (
	*BrickAllocation, error) {

	if index < 0 || index >= bs.SetSize {
		return nil, fmt.Errorf(
			"brick replace index out of bounds (got %v, set size %v)",
			index, bs.SetSize)
	}
	logger.Info("Replace brick in brick set %v with index %v",
		bs, index)

	// we return a brick allocation for symmetry with PlaceAll
	// but it only contains one pair of sets
	r := &BrickAllocation{
		BrickSets:  []*BrickSet{NewBrickSet(bs.SetSize)},
		DeviceSets: []*DeviceSet{NewDeviceSet(bs.SetSize)},
	}
	wbs := r.BrickSets[0]
	wds := r.DeviceSets[0]

	dscan, err := bp.Scanner(dsrc)
	if err != nil {
		return r, err
	}
	defer dscan.Close()

	// copy input brick set to working brick set and get the
	// corresponding device entries for the device set
	for i, b := range bs.Bricks {
		d, err := dsrc.Device(b.Info.DeviceId)
		if err != nil {
			return r, err
		}
		wbs.Insert(i, b)
		wds.Insert(i, d)
	}
	aopts := newArbiterOpts(opts)
	err = bp.placeBrickInSet(dsrc, dscan, aopts, pred, wbs, wds, index)
	return r, err
}

// newSets returns a new fully populated pair of brick and device sets.
// If new sets can not be placed err will be non-nil.
func (bp *ArbiterBrickPlacer) newSets(
	dsrc DeviceSource,
	opts PlacementOpts,
	pred DeviceFilter) (*BrickSet, *DeviceSet, error) {

	ssize := opts.SetSize()
	bs := NewBrickSet(ssize)
	ds := NewDeviceSet(ssize)
	dscan, err := bp.Scanner(dsrc)
	if err != nil {
		return nil, nil, err
	}
	defer dscan.Close()

	for index := 0; index < ssize; index++ {
		aopts := newArbiterOpts(opts)
		if e := aopts.discount(index); e != nil {
			return bs, ds, e
		}
		err := bp.placeBrickInSet(dsrc, dscan, aopts, pred, bs, ds, index)
		if err != nil {
			return bs, ds, err
		}
	}
	return bs, ds, nil
}

// placeBrickInSet uses the device scanner to find a device suitable
// for a new brick at the given index. If no devices can be found for
// the brick err will be non-nil.
func (bp *ArbiterBrickPlacer) placeBrickInSet(
	dsrc DeviceSource,
	dscan *arbiterDeviceScanner,
	opts *arbiterOpts,
	pred DeviceFilter,
	bs *BrickSet,
	ds *DeviceSet,
	index int) error {

	logger.Info("Placing brick in brick set at position %v", index)
	for deviceId := range dscan.Scan(index) {

		device, err := dsrc.Device(deviceId)
		if err != nil {
			return err
		}

		err = bp.tryPlaceBrickOnDevice(
			opts, pred, bs, ds, index, device)
		switch err {
		case tryPlaceAgain:
			continue
		case nil:
			logger.Debug("Placed brick at index %v on device %v",
				index, deviceId)
			return nil
		default:
			return err
		}
	}

	// we exhausted all possible devices for this brick
	logger.Debug("Can not find any device for brick (index=%v)", index)
	return ErrNoSpace
}

// tryPlaceBrickOnDevice attempts to place a brick on the given device.
// If placement is successful the brick and device sets are updated,
// and the error is nil.
// If placement fails then tryPlaceAgain error is returned.
func (bp *ArbiterBrickPlacer) tryPlaceBrickOnDevice(
	opts *arbiterOpts,
	pred DeviceFilter,
	bs *BrickSet,
	ds *DeviceSet,
	index int,
	device *DeviceEntry) error {

	logger.Debug("Trying to place brick on device %v", device.Info.Id)

	for i, b := range bs.Bricks {
		// do not check the brick in the brick set for the current
		// index. If this is a new brick set we won't have the index
		// populated. If this is a replace, we will have the old brick
		// at the index and we are OK with re-using its node (as the
		// standard placer does)
		if i == index {
			continue
		}
		if b.Info.NodeId == device.NodeId {
			// this node is used by an existing brick in the set
			// we can not use this device
			logger.Debug("Node %v already in use by brick set (device %v)",
				device.NodeId, device.Info.Id)
			return tryPlaceAgain
		}
	}

	if pred != nil && !pred(bs, device) {
		logger.Debug("Device %v rejected by predicate function", device.Info.Id)
		return tryPlaceAgain
	}

	// Try to allocate a brick on this device
	origBrickSize, snapFactor := opts.o.BrickSizes()
	brickSize := opts.brickSize
	if brickSize != origBrickSize {
		logger.Info("Placing brick with discounted size: %v", brickSize)
	}
	brick := device.NewBrickEntry(brickSize, snapFactor,
		opts.o.BrickGid(), opts.o.BrickOwner())
	if brick == nil {
		logger.Debug(
			"Unable to place a brick of size %v & factor %v on device %v",
			brickSize, snapFactor, device.Info.Id)
		return tryPlaceAgain
	}

	device.BrickAdd(brick.Id())
	bs.Insert(index, brick)
	ds.Insert(index, device)
	return nil
}

type arbiterDeviceScanner struct {
	arbiterDevs <-chan string
	arbiterDone chan struct{}
	dataDevs    <-chan string
	dataDone    chan struct{}
}

// Scanner returns a pointer to an arbiterDeviceScanner helper object.
// This object can be used to range over the devices that a brick
// may be placed on. The .Close method must be called to release
// resources associated with this object.
func (bp *ArbiterBrickPlacer) Scanner(dsrc DeviceSource) (
	*arbiterDeviceScanner, error) {

	dataRing := NewSimpleAllocatorRing()
	arbiterRing := NewSimpleAllocatorRing()
	dnl, err := dsrc.Devices()
	if err != nil {
		return nil, err
	}
	for _, dan := range dnl {
		sd := &SimpleDevice{
			zone:     dan.Node.Info.Zone,
			nodeId:   dan.Node.Info.Id,
			deviceId: dan.Device.Info.Id,
		}
		// it is perfectly fine for a device to host data & arbiter
		// bricks if it is so configured. Thus both the following
		// blocks may be true.
		if bp.canHostArbiter(dan.Device) {
			arbiterRing.Add(sd)
		}
		if bp.canHostData(dan.Device) {
			dataRing.Add(sd)
		}
	}

	id := utils.GenUUID()
	dataDevs, dataDone := make(chan string), make(chan struct{})
	generateDevices(dataRing.GetDeviceList(id), dataDevs, dataDone)
	arbiterDevs, arbiterDone := make(chan string), make(chan struct{})
	generateDevices(arbiterRing.GetDeviceList(id), arbiterDevs, arbiterDone)
	return &arbiterDeviceScanner{
		arbiterDevs: arbiterDevs,
		arbiterDone: arbiterDone,
		dataDevs:    dataDevs,
		dataDone:    dataDone,
	}, nil
}

// Close releases the resources held by the scanner.
func (dscan *arbiterDeviceScanner) Close() {
	close(dscan.arbiterDone)
	close(dscan.dataDone)
}

// Scan returns a channel that may be ranged over for eligible devices
// for a brick in a brick set with the position specified by index.
func (dscan *arbiterDeviceScanner) Scan(index int) <-chan string {
	// currently this is hard-coded such that the index of
	// an arbiter brick is always two (the 3rd brick in the set of three)
	// In the future we may want to be smarter here, but this
	// works for now.
	if index == arbiter_index {
		return dscan.arbiterDevs
	}
	return dscan.dataDevs
}

func discountBrickSize(size, discountSize uint64) (uint64, error) {
	if size < discountSize {
		return 0, fmt.Errorf(
			"Brick size (%v) too small for arbiter (discount size %v)",
			size, discountSize)
	}
	return (size / discountSize), nil
}
