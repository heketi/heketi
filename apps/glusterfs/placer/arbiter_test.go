//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package placer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/heketi/tests"

	"github.com/heketi/heketi/pkg/utils"
)

var (
	// define a mock function for always accepting a device as
	// OK for arbiter or data bricks
	hostTrue = func(PlacerDevice, DeviceSource) bool {
		return true
	}
)

type MockDevice struct {
	Info struct {
		Id      string
		Name    string
		Storage struct {
			Total uint64
			Free  uint64
		}
	}
	NodeId string
}

func (d *MockDevice) Id() string {
	return d.Info.Id
}

func (d *MockDevice) ParentNodeId() string {
	return d.NodeId
}

func (d *MockDevice) NewBrick(brickSize uint64,
	snapFactor float64,
	brickGid int64,
	owner string) PlacerBrick {

	// the constant 50 exists to mimic the behavior of the
	// overhead added onto the brick size during real brick
	// sizing
	if (brickSize + 50) > d.Info.Storage.Free {
		return nil
	}

	b := &MockBrick{}
	b.Info.Id = utils.GenUUID()
	b.Info.NodeId = d.NodeId
	b.Info.DeviceId = d.Id()
	b.Info.Size = brickSize
	return b
}

func (d *MockDevice) BrickAdd(string) {
	return
}

type MockNode struct {
	Info struct {
		Id        string
		Zone      int
		Manage    string
		Storage   string
		ClusterId string
	}
	Devices []string
}

func (n *MockNode) Id() string {
	return n.Info.Id
}

func (n *MockNode) Zone() int {
	return n.Info.Zone
}

type MockBrick struct {
	Info struct {
		Id       string
		DeviceId string
		NodeId   string
		Size     uint64
	}
}

func (b *MockBrick) Id() string {
	return b.Info.Id
}

func (b *MockBrick) DeviceId() string {
	return b.Info.DeviceId
}

func (b *MockBrick) NodeId() string {
	return b.Info.NodeId
}

func (b *MockBrick) SetId(id string) {
	b.Info.Id = id
}

func (b *MockBrick) Valid() bool {
	return b != nil
}

type TestDeviceSource struct {
	devices      map[string]*MockDevice
	nodes        map[string]*MockNode
	devicesError error
}

func NewTestDeviceSource() *TestDeviceSource {
	return &TestDeviceSource{
		devices: map[string]*MockDevice{},
		nodes:   map[string]*MockNode{},
	}
}

func (tds *TestDeviceSource) AddDevice(d *MockDevice) {
	tds.devices[d.Info.Id] = d
}

func (tds *TestDeviceSource) AddNode(n *MockNode) {
	tds.nodes[n.Info.Id] = n
}

func (tds *TestDeviceSource) QuickAdd(
	nodeId, deviceId, dname string, size uint64) {

	tds.MultiAdd(nodeId)(deviceId, dname, size)
}

func (tds *TestDeviceSource) MultiAdd(nodeId string) func(string, string, uint64) {

	n := &MockNode{}
	n.Info.Id = nodeId
	n.Info.Zone = 1
	n.Info.Manage = "mng-" + nodeId
	n.Info.Storage = "stor-" + nodeId
	n.Info.ClusterId = "0000000000c"
	tds.AddNode(n)

	return func(deviceId, dname string, size uint64) {
		d := &MockDevice{}
		d.Info.Id = deviceId
		d.Info.Name = dname
		d.Info.Storage.Total = size
		d.Info.Storage.Free = size
		d.NodeId = nodeId

		n.Devices = append(n.Devices, d.Info.Id)
		tds.AddDevice(d)
	}
}

func (tds *TestDeviceSource) Devices() ([]DeviceAndNode, error) {
	if tds.devicesError != nil {
		return nil, tds.devicesError
	}
	valid := [](DeviceAndNode){}
	for _, node := range tds.nodes {
		for _, deviceId := range node.Devices {
			device, ok := tds.devices[deviceId]
			if !ok {
				return nil, errNotFound
			}
			valid = append(valid, DeviceAndNode{
				Device: device,
				Node:   node,
			})
		}
	}
	return valid, nil
}

func (tds *TestDeviceSource) Device(id string) (PlacerDevice, error) {
	if device, ok := tds.devices[id]; ok {
		return device, nil
	}
	return nil, errNotFound
}

func (tds *TestDeviceSource) Node(id string) (PlacerNode, error) {
	if node, ok := tds.nodes[id]; ok {
		return node, nil
	}
	return nil, errNotFound
}

type TestPlacementOpts struct {
	brickSize       uint64
	brickSnapFactor float64
	brickOwner      string
	brickGid        int64
	setSize         int
	setCount        int
	averageFileSize uint64
}

func (tpo *TestPlacementOpts) BrickSizes() (uint64, float64) {
	return tpo.brickSize, tpo.brickSnapFactor
}

func (tpo *TestPlacementOpts) BrickOwner() string {
	return tpo.brickOwner
}

func (tpo *TestPlacementOpts) BrickGid() int64 {
	return tpo.brickGid
}

func (tpo *TestPlacementOpts) SetSize() int {
	return tpo.setSize
}

func (tpo *TestPlacementOpts) SetCount() int {
	return tpo.setCount
}

func (tpo *TestPlacementOpts) AverageFileSize() uint64 {
	return tpo.averageFileSize
}

func TestTestDeviceSource(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		1100)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		1200)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		1300)

	tests.Assert(t, len(dsrc.devices) == 3,
		"expected len(dsrc.devices) == 3, got:", len(dsrc.devices))
	tests.Assert(t, len(dsrc.nodes) == 3,
		"expected len(dsrc.nodes) == 3, got:", len(dsrc.nodes))

	pd, err := dsrc.Device("22222222")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	d := pd.(*MockDevice)
	tests.Assert(t, d.Info.Storage.Total == 1200,
		"expected d.Info.Storage.Total == 1200, got:", d.Info.Storage.Total)

	pd, err = dsrc.Device("10000000")
	tests.Assert(t, err == errNotFound, "expected err == errNotFound, got:", err)

	pn, err := dsrc.Node("30000000")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	n := pn.(*MockNode)
	tests.Assert(t, n.Info.Manage == "mng-30000000",
		"expected n.Info.Hostnames.Manage[0] == \"mng-30000000\", got:",
		n.Info.Manage)

	pn, err = dsrc.Node("abcdefgh")
	tests.Assert(t, err == errNotFound, "expected err == errNotFound, got:", err)

	dnl, err := dsrc.Devices()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(dnl) == 3,
		"expected len(dnl) == 3, got:", len(dnl))
}

func TestTestDeviceSourceMultiAdd(t *testing.T) {
	dsrc := NewTestDeviceSource()
	addDev := dsrc.MultiAdd("abcd")
	addDev("d1", "/dev/x1", 10)
	addDev("d2", "/dev/x2", 20)
	addDev("d3", "/dev/x3", 30)
	addDev = dsrc.MultiAdd("foo")
	addDev("d4", "/dev/x1", 40)
	addDev("d5", "/dev/x2", 50)
	dsrc.MultiAdd("bar")("d6", "/dev/x1", 60)

	tests.Assert(t, len(dsrc.devices) == 6,
		"expected len(dsrc.devices) == 6, got:", len(dsrc.devices))
	tests.Assert(t, len(dsrc.nodes) == 3,
		"expected len(dsrc.nodes) == 3, got:", len(dsrc.nodes))

	dnl, err := dsrc.Devices()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(dnl) == 6,
		"expected len(dnl) == 6, got:", len(dnl))
}

func TestArbiterBrickPlacer(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		11000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		12000)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		13000)
	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer(hostTrue, hostTrue)
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))

	tests.Assert(t, ba.BrickSets[0].Full(),
		"expected ba.BrickSets[0].Full() to be true, was false")
	tests.Assert(t, ba.BrickSets[0].SetSize == 3,
		"expected ba.BrickSets[0].SetSize == 3, got:",
		ba.BrickSets[0].SetSize)

	bi0 := ba.BrickSets[0].Bricks[0].(*MockBrick).Info
	bi1 := ba.BrickSets[0].Bricks[1].(*MockBrick).Info
	bi2 := ba.BrickSets[0].Bricks[2].(*MockBrick).Info
	tests.Assert(t, bi0.Size == bi1.Size,
		"expected bi0.Size == bi1.Size, got:",
		bi0.Size, bi1.Size)
	tests.Assert(t, bi0.Size > bi2.Size,
		"expected bi0.Size > bi2.Size, got:",
		bi0.Size, bi2.Size)
}

func TestArbiterBrickPlacerTooSmall(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		810)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		820)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		830)
	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer(hostTrue, hostTrue)
	_, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == ErrNoDevices, "expected err == ErrNoSpace, got:", err)
}

func TestArbiterBrickPlacerDevicesFail(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.devicesError = fmt.Errorf("Zonk!")

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer(hostTrue, hostTrue)
	_, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == dsrc.devicesError,
		"expected err == dsrc.devicesError, got:", err)
}

func TestArbiterBrickPlacerPredicateBlock(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		11000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		12000)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		13000)
	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer(hostTrue, hostTrue)
	pred := func(bs *BrickSet, d PlacerDevice) bool {
		return false
	}
	_, err := abplacer.PlaceAll(dsrc, opts, pred)
	tests.Assert(t, err == ErrNoDevices, "expected err == ErrNoSpace, got:", err)
}

func TestArbiterBrickPlacerBrickOnArbiterDevice(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		21000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		22000)
	dsrc.QuickAdd(
		"30000000",
		"a3333333",
		"/dev/foobar",
		23000)
	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	canHostArbiter := func(d PlacerDevice, ds DeviceSource) bool {
		return d.Id()[0] == 'a'
	}
	canHostData := func(d PlacerDevice, ds DeviceSource) bool {
		return !canHostArbiter(d, ds)
	}
	abplacer := NewArbiterBrickPlacer(canHostArbiter, canHostData)
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	bi2 := ba.BrickSets[0].Bricks[2].(*MockBrick).Info
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))
	tests.Assert(t, bi2.DeviceId == "a3333333",
		`expected bi2.DeviceId == "a3333333", got:`,
		bi2.DeviceId)
	di2 := ba.DeviceSets[0].Devices[2].(*MockDevice).Info
	tests.Assert(t, di2.Id == "a3333333",
		`expected di2.Id == "a3333333", got`,
		di2.Id)
}

func TestArbiterBrickPlacerBrickThreeSets(t *testing.T) {
	dsrc := NewTestDeviceSource()
	addDev := dsrc.MultiAdd("10000000")
	addDev("11111111", "/dev/d1", 10001)
	addDev("21111111", "/dev/d2", 10002)
	addDev("31111111", "/dev/d3", 10003)
	addDev = dsrc.MultiAdd("20000000")
	addDev("41111111", "/dev/d1", 10001)
	addDev("51111111", "/dev/d2", 10002)
	addDev("61111111", "/dev/d3", 10003)
	addDev = dsrc.MultiAdd("30000000")
	addDev("71111111", "/dev/d1", 10001)
	addDev("81111111", "/dev/d2", 10002)
	addDev("91111111", "/dev/d3", 10003)

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        3,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer(hostTrue, hostTrue)
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 3,
		"expected len(ba.BrickSets) == 3, got:", len(ba.BrickSets))
}

func TestArbiterBrickPlacerBrickThreeSetsOnArbiterDevice(t *testing.T) {
	dsrc := NewTestDeviceSource()
	addDev := dsrc.MultiAdd("10000000")
	// data nodes
	addDev("11111111", "/dev/d1", 10001)
	addDev("21111111", "/dev/d2", 10002)
	addDev = dsrc.MultiAdd("20000000")
	addDev("31111111", "/dev/d1", 10001)
	addDev("41111111", "/dev/d2", 10002)
	addDev = dsrc.MultiAdd("30000000")
	addDev("51111111", "/dev/d1", 10001)
	addDev("61111111", "/dev/d2", 10002)
	addDev = dsrc.MultiAdd("40000000")
	addDev("71111111", "/dev/d1", 10001)
	addDev("81111111", "/dev/d2", 10002)
	// arbiter nodes
	addDev = dsrc.MultiAdd("50000000")
	addDev("a1111111", "/dev/d1", 10001)
	addDev = dsrc.MultiAdd("60000000")
	addDev("a2111111", "/dev/d1", 10001)
	addDev = dsrc.MultiAdd("70000000")
	addDev("a3111111", "/dev/d1", 10001)
	// the above configuration is pretty artificial and reflects
	// a downside to the current approach. because of the
	// non-deterministic way the ring provides devices and
	// the (hard) requirement not to reuse a node within the brick
	// set its fairly easy with small devices to run into a situation
	// where the placement fails even though the configuration could
	// have hosted the volume. There are things we can do to
	// improve this but not now and not for this test. :-\

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        3,
		averageFileSize: 64 * KB,
	}

	canHostArbiter := func(d PlacerDevice, ds DeviceSource) bool {
		return d.Id()[0] == 'a'
	}
	canHostData := func(d PlacerDevice, ds DeviceSource) bool {
		return !canHostArbiter(d, ds)
	}
	abplacer := NewArbiterBrickPlacer(canHostArbiter, canHostData)
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 3,
		"expected len(ba.BrickSets) == 3, got:", len(ba.BrickSets))
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			brickDeviceId := ba.BrickSets[i].Bricks[j].(*MockBrick).Info.DeviceId
			prefixA := (brickDeviceId[0] == 'a')
			if j == 2 {
				tests.Assert(t, prefixA, "expected prefixA true on index", j)
			} else {
				tests.Assert(t, !prefixA, "expected prefixA false on index", j)
			}
		}
	}
}

func TestArbiterBrickPlacerSimpleReplace(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		21000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		22000)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		23000)
	dsrc.QuickAdd(
		"40000000",
		"44444444",
		"/dev/foobar",
		23000)

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer(hostTrue, hostTrue)
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))
	tests.Assert(t, len(ba.BrickSets[0].Bricks) == 3,
		"expected len(ba.BrickSets[0].Bricks) == 3, got:",
		len(ba.BrickSets[0].Bricks))

	ba2, err := abplacer.Replace(dsrc, opts, nil, ba.BrickSets[0], 0)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba2.BrickSets) == 1,
		"expected len(ba2.BrickSets) == 1, got:", len(ba2.BrickSets))
	tests.Assert(t, len(ba2.BrickSets[0].Bricks) == 3,
		"expected len(ba2.BrickSets[0].Bricks) == 3, got:",
		len(ba2.BrickSets[0].Bricks))

	bs1 := ba.BrickSets[0]
	bs2 := ba2.BrickSets[0]
	// we replaced the 1st brick, thus it should differ
	tests.Assert(t,
		bs1.Bricks[0].Id() != bs2.Bricks[0].Id(),
		"expected bs1.Bricks[0].Id() == bs2.Bricks[0].Id(), got:",
		bs1.Bricks[0].Id(), bs2.Bricks[0].Id())
	// the remaining bricks will be the same
	tests.Assert(t,
		bs1.Bricks[1].Id() == bs2.Bricks[1].Id(),
		"expected bs1.Bricks[1].Id() == bs2.Bricks[1].Id(), got:",
		bs1.Bricks[1].Id(), bs2.Bricks[1].Id())
	tests.Assert(t,
		bs1.Bricks[2].Id() == bs2.Bricks[2].Id(),
		"expected bs1.Bricks[2].Id() == bs2.Bricks[2].Id(), got:",
		bs1.Bricks[2].Id(), bs2.Bricks[2].Id())
}

func TestArbiterBrickPlacerReplaceIndexOOB(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		21000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		22000)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		23000)

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer(hostTrue, hostTrue)
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))
	tests.Assert(t, len(ba.BrickSets[0].Bricks) == 3,
		"expected len(ba.BrickSets[0].Bricks) == 3, got:",
		len(ba.BrickSets[0].Bricks))

	_, err = abplacer.Replace(dsrc, opts, nil, ba.BrickSets[0], -1)
	tests.Assert(t, strings.Contains(err.Error(), "out of bounds"),
		`expected strings.Contains(err.Error(), "out of bounds"), got:`,
		err.Error())

	_, err = abplacer.Replace(dsrc, opts, nil, ba.BrickSets[0], 9)
	tests.Assert(t, strings.Contains(err.Error(), "out of bounds"),
		`expected strings.Contains(err.Error(), "out of bounds"), got:`,
		err.Error())
}

func TestArbiterBrickPlacerReplaceDevicesFail(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		21000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		22000)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		23000)
	dsrc.QuickAdd(
		"40000000",
		"44444444",
		"/dev/foobar",
		23000)

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer(hostTrue, hostTrue)
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))
	tests.Assert(t, len(ba.BrickSets[0].Bricks) == 3,
		"expected len(ba.BrickSets[0].Bricks) == 3, got:",
		len(ba.BrickSets[0].Bricks))

	dsrc.devicesError = fmt.Errorf("Zonk!")
	_, err = abplacer.Replace(dsrc, opts, nil, ba.BrickSets[0], 0)
	tests.Assert(t, err == dsrc.devicesError,
		"expected err == dsrc.devicesError, got:", err)
}

func TestArbiterBrickPlacerReplaceTooFew(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		21000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		22000)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		23000)

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer(hostTrue, hostTrue)
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))
	tests.Assert(t, len(ba.BrickSets[0].Bricks) == 3,
		"expected len(ba.BrickSets[0].Bricks) == 3, got:",
		len(ba.BrickSets[0].Bricks))

	pred := func(bs *BrickSet, d PlacerDevice) bool {
		return ba.BrickSets[0].Bricks[0].DeviceId() != d.Id()
	}
	_, err = abplacer.Replace(dsrc, opts, pred, ba.BrickSets[0], 0)
	tests.Assert(t, err == ErrNoDevices,
		"expected err == ErrNoSpace, got:", err)
}

func TestArbiterBrickPlacerReplaceTooFewArbiter(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		21000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		22000)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		23000)
	dsrc.QuickAdd(
		"40000000",
		"44444444",
		"/dev/foobar",
		24000)
	dsrc.QuickAdd(
		"50000000",
		"a5555555",
		"/dev/foobar",
		25000)
	// we have enough devices for a generic replace but not
	// when we're limited to certain devices

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	canHostArbiter := func(d PlacerDevice, ds DeviceSource) bool {
		return d.Id()[0] == 'a'
	}
	canHostData := func(d PlacerDevice, ds DeviceSource) bool {
		return !canHostArbiter(d, ds)
	}
	abplacer := NewArbiterBrickPlacer(canHostArbiter, canHostData)

	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))
	tests.Assert(t, len(ba.BrickSets[0].Bricks) == 3,
		"expected len(ba.BrickSets[0].Bricks) == 3, got:",
		len(ba.BrickSets[0].Bricks))

	// this will fail because we have no more "arbiter devices"
	pred := func(bs *BrickSet, d PlacerDevice) bool {
		return ba.BrickSets[0].Bricks[2].DeviceId() != d.Id()
	}
	_, err = abplacer.Replace(dsrc, opts, pred, ba.BrickSets[0], 2)
	tests.Assert(t, err == ErrNoDevices,
		"expected err == ErrNoSpace, got:", err)

	// this one will work because the free device is not arbiter
	// and the 1 position is a data brick
	ba2, err := abplacer.Replace(dsrc, opts, nil, ba.BrickSets[0], 1)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba2.BrickSets) == 1,
		"expected len(ba2.BrickSets) == 1, got:", len(ba2.BrickSets))
	tests.Assert(t, len(ba2.BrickSets[0].Bricks) == 3,
		"expected len(ba2.BrickSets[0].Bricks) == 3, got:",
		len(ba2.BrickSets[0].Bricks))
}
