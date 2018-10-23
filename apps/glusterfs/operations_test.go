package glusterfs

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
	"github.com/heketi/tests"

	"github.com/gorilla/mux"
)

func TestDeviceRemoveOperationEmpty(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// grab a device
	var d *DeviceEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		dl, err := DeviceList(tx)
		if err != nil {
			return err
		}
		for _, id := range dl {
			d, err = NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			break
		}
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// because there are no bricks on this device it can be disabled
	// instantly and there are no pending ops for it in the db
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})

	err = dro.Exec(app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = dro.Finalize()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func TestDeviceRemoveOperationWithBricks(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create two volumes
	for i := 0; i < 5; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a devices that has bricks
	var d *DeviceEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		dl, err := DeviceList(tx)
		if err != nil {
			return err
		}
		for _, id := range dl {
			d, err = NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			if len(d.Bricks) > 0 {
				return nil
			}
		}
		t.Fatalf("should have at least one device with bricks")
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// because there were bricks on this device it needs to perform
	// a full "operation cycle"
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}

	err = dro.Exec(app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// operation is not over. we should still have a pending op
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	err = dro.Finalize()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// operation is over. we should _not_ have a pending op now
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	// our d should be w/o bricks and be in failed state
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 0,
		"expected len(d.Bricks) == 0, got:", len(d.Bricks))
	tests.Assert(t, d.State == api.EntryStateFailed)
}

func TestDeviceRemoveOperationTooFewDevices(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create two volumes
	for i := 0; i < 5; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a devices that has bricks
	var d *DeviceEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		dl, err := DeviceList(tx)
		if err != nil {
			return err
		}
		for _, id := range dl {
			d, err = NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			if len(d.Bricks) > 0 {
				return nil
			}
		}
		t.Fatalf("should have at least one device with bricks")
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// because there were bricks on this device it needs to perform
	// a full "operation cycle"
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}

	err = dro.Exec(app.executor)
	tests.Assert(t, strings.Contains(err.Error(), ErrNoReplacement.Error()),
		"expected strings.Contains(err.Error(), ErrNoReplacement.Error()), got:",
		err.Error())

	// operation is not over. we should still have a pending op
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	err = dro.Rollback(app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// operation is over. we should _not_ have a pending op now
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	// our d should be in the original state because the exec failed
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) > 0,
		"expected len(d.Bricks) > 0, got:", len(d.Bricks))
	tests.Assert(t, d.State == api.EntryStateOffline)
}

func TestDeviceRemoveOperationOtherPendingOps(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create two volumes
	for i := 0; i < 4; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a devices that has bricks
	var d *DeviceEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		dl, err := DeviceList(tx)
		if err != nil {
			return err
		}
		for _, id := range dl {
			d, err = NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			if len(d.Bricks) > 0 {
				return nil
			}
		}
		t.Fatalf("should have at least one device with bricks")
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// now start a volume create operation but don't finish it
	vol := NewVolumeEntryFromRequest(vreq)
	vc := NewVolumeCreateOperation(vol, app.db)
	err = vc.Build()
	tests.Assert(t, err == nil, "expected e == nil, got", err)
	// we should have one pending operation
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db)
	err = dro.Build()
	tests.Assert(t, err == ErrConflict, "expected err == ErrConflict, got:", err)

	// we should have one pending operation (the volume create)
	app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})
}

// TestDeviceRemoveOperationMultipleRequests tests that
// the system fails gracefully if a remove device request
// comes in while an existing operation is already in progress.
func TestDeviceRemoveOperationMultipleRequests(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create volumes
	for i := 0; i < 4; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a device that has bricks
	var d *DeviceEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		dl, err := DeviceList(tx)
		if err != nil {
			return err
		}
		for _, id := range dl {
			d, err = NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			if len(d.Bricks) > 0 {
				return nil
			}
		}
		t.Fatalf("should have at least one device with bricks")
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// perform the build step of one remove operation
	dro := NewDeviceRemoveOperation(d.Info.Id, app.db)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// perform the build step of a 2nd remove operation
	// we can "fake' it this way in a test because the transactions
	// that cover the Build steps are effectively serializing
	// these actions.
	dro2 := NewDeviceRemoveOperation(d.Info.Id, app.db)
	err = dro2.Build()
	tests.Assert(t, err == ErrConflict, "expected err == ErrConflict, got:", err)

	// we should have one pending operation (the device remove)
	app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

}

type testOperation struct {
	label    string
	rurl     string
	retryMax int
	build    func() error
	exec     func() error
	finalize func() error
	rollback func() error
}

func (o *testOperation) Label() string {
	return o.label
}

func (o *testOperation) ResourceUrl() string {
	return o.rurl
}

func (o *testOperation) MaxRetries() int {
	return o.retryMax
}

func (o *testOperation) Build() error {
	if o.build == nil {
		return nil
	}
	return o.build()
}

func (o *testOperation) Exec(executor executors.Executor) error {
	if o.exec == nil {
		return nil
	}
	return o.exec()
}

func (o *testOperation) Rollback(executor executors.Executor) error {
	if o.rollback == nil {
		return nil
	}
	return o.rollback()
}

func (o *testOperation) Finalize() error {
	if o.finalize == nil {
		return nil
	}
	return o.finalize()
}

func TestAsyncHttpOperationOK(t *testing.T) {
	o := &testOperation{}
	o.rurl = "/myresource"
	testAsyncHttpOperation(t, o, func(t *testing.T, url string) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		r, err := client.Get(url + "/app")
		tests.Assert(t, r.StatusCode == http.StatusAccepted)
		tests.Assert(t, err == nil)
		location, err := r.Location()
		tests.Assert(t, err == nil)

		for done := false; !done; {
			time.Sleep(time.Millisecond)
			r, err = client.Get(location.String())
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			switch r.StatusCode {
			case http.StatusSeeOther:
				location, err = r.Location()
				tests.Assert(t, err == nil, "expected err == nil, got", err)
				tests.Assert(t, strings.Contains(location.String(), "/myresource"),
					`expected trings.Contains(location.String(), "/myresource") got:`,
					location.String())
			case http.StatusOK:
				if r.ContentLength > 0 {
					body, err := ioutil.ReadAll(r.Body)
					r.Body.Close()
					tests.Assert(t, err == nil)
					tests.Assert(t, string(body) == "HelloWorld")
					done = true
				} else {
					t.Fatalf("unexpected content length %v", r.ContentLength)
				}
			default:
				t.Fatalf("unexpected http return code %v", r.StatusCode)
			}
		}
	})
}

func TestAsyncHttpOperationBuildFailure(t *testing.T) {
	o := &testOperation{}
	o.rurl = "/myresource"
	o.build = func() error {
		return fmt.Errorf("buildfail")
	}
	testAsyncHttpOperation(t, o, func(t *testing.T, url string) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		r, err := client.Get(url + "/app")
		tests.Assert(t, err == nil, "expected err == nil, got", err)
		tests.Assert(t, r.StatusCode == http.StatusInternalServerError)
	})
}

func TestAsyncHttpOperationExecFailure(t *testing.T) {
	o := &testOperation{}
	o.rurl = "/myresource"
	o.exec = func() error {
		return fmt.Errorf("execfail")
	}
	testAsyncHttpOperation(t, o, func(t *testing.T, url string) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		r, err := client.Get(url + "/app")
		tests.Assert(t, err == nil, "expected err == nil, got", err)
		tests.Assert(t, r.StatusCode == http.StatusAccepted)
		location, err := r.Location()
		tests.Assert(t, err == nil)

		for done := false; !done; {
			time.Sleep(time.Millisecond)
			r, err = client.Get(location.String())
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			switch r.StatusCode {
			case http.StatusSeeOther:
				location, err = r.Location()
				tests.Assert(t, err == nil, "expected err == nil, got", err)
				tests.Assert(t, strings.Contains(location.String(), "/myresource"),
					`expected trings.Contains(location.String(), "/myresource") got:`,
					location.String())
			case http.StatusInternalServerError:
				if r.ContentLength > 0 {
					body, err := ioutil.ReadAll(r.Body)
					r.Body.Close()
					tests.Assert(t, err == nil)
					s := string(body)
					tests.Assert(t, strings.Contains(s, "execfail"),
						`expected strings.Contains(s, "execfail"), got:`, s)
					done = true
				} else {
					t.Fatalf("unexpected content length %v", r.ContentLength)
				}
			default:
				t.Fatalf("unexpected http return code %v", r.StatusCode)
			}
		}
	})
}

func TestAsyncHttpOperationRollbackFailure(t *testing.T) {
	o := &testOperation{}
	o.rurl = "/myresource"
	o.exec = func() error {
		return fmt.Errorf("execfail")
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return fmt.Errorf("rollbackfail")
	}
	testAsyncHttpOperation(t, o, func(t *testing.T, url string) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		r, err := client.Get(url + "/app")
		tests.Assert(t, err == nil, "expected err == nil, got", err)
		tests.Assert(t, r.StatusCode == http.StatusAccepted)
		location, err := r.Location()
		tests.Assert(t, err == nil)

		for done := false; !done; {
			time.Sleep(time.Millisecond)
			r, err = client.Get(location.String())
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			switch r.StatusCode {
			case http.StatusSeeOther:
				location, err = r.Location()
				tests.Assert(t, err == nil, "expected err == nil, got", err)
				tests.Assert(t, strings.Contains(location.String(), "/myresource"),
					`expected trings.Contains(location.String(), "/myresource") got:`,
					location.String())
			case http.StatusInternalServerError:
				if r.ContentLength > 0 {
					body, err := ioutil.ReadAll(r.Body)
					r.Body.Close()
					tests.Assert(t, err == nil)
					s := string(body)
					tests.Assert(t, strings.Contains(s, "execfail"),
						`expected strings.Contains(s, "execfail"), got:`, s)
					done = true
				} else {
					t.Fatalf("unexpected content length %v", r.ContentLength)
				}
			default:
				t.Fatalf("unexpected http return code %v", r.StatusCode)
			}
		}
	})
	tests.Assert(t, rollback_cc == 1, "expected rollback_cc == 1, got:", rollback_cc)
}

func TestAsyncHttpOperationFinalizeFailure(t *testing.T) {
	o := &testOperation{}
	o.rurl = "/myresource"
	o.finalize = func() error {
		return fmt.Errorf("finfail")
	}
	testAsyncHttpOperation(t, o, func(t *testing.T, url string) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		r, err := client.Get(url + "/app")
		tests.Assert(t, err == nil, "expected err == nil, got", err)
		tests.Assert(t, r.StatusCode == http.StatusAccepted)
		location, err := r.Location()
		tests.Assert(t, err == nil)

		for done := false; !done; {
			time.Sleep(time.Millisecond)
			r, err = client.Get(location.String())
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			switch r.StatusCode {
			case http.StatusSeeOther:
				location, err = r.Location()
				tests.Assert(t, err == nil, "expected err == nil, got", err)
				tests.Assert(t, strings.Contains(location.String(), "/myresource"),
					`expected trings.Contains(location.String(), "/myresource") got:`,
					location.String())
			case http.StatusInternalServerError:
				if r.ContentLength > 0 {
					body, err := ioutil.ReadAll(r.Body)
					r.Body.Close()
					tests.Assert(t, err == nil)
					s := string(body)
					tests.Assert(t, strings.Contains(s, "finfail"),
						`expected strings.Contains(s, "finfail"), got:`, s)
					done = true
				} else {
					t.Fatalf("unexpected content length %v", r.ContentLength)
				}
			default:
				t.Fatalf("unexpected http return code %v", r.StatusCode)
			}
		}
	})
}

func testAsyncHttpOperation(t *testing.T,
	o Operation,
	testFunc func(*testing.T, string)) {

	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc("/queue/{id}", app.asyncManager.HandlerStatus).Methods("GET")
	router.HandleFunc("/myresource", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "HelloWorld")
	}).Methods("GET")

	router.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		if x := AsyncHttpOperation(app, w, r, o); x != nil {
			http.Error(w, x.Error(), http.StatusInternalServerError)
		}
	}).Methods("GET")

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	testFunc(t, ts.URL)
}

func TestRunOperationRollbackFailure(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{}
	o.rurl = "/myresource"
	o.exec = func() error {
		return fmt.Errorf("execfail")
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return fmt.Errorf("rollbackfail")
	}
	e := RunOperation(o, app.executor)
	// even if rollback fails we expect the error from Exec
	tests.Assert(t, strings.Contains(e.Error(), "execfail"),
		`expected strings.Contains(e.Error(), "execfail"), got:`, e)
	// check that rollback got called
	tests.Assert(t, rollback_cc == 1,
		"expected rollback_cc == 1, got:", rollback_cc)
}

func TestRunOperationFinalizeFailure(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{}
	o.label = "Funky Fresh"
	o.rurl = "/myresource"
	o.finalize = func() error {
		return fmt.Errorf("finfail")
	}

	e := RunOperation(o, app.executor)
	// check error from finalize
	tests.Assert(t, strings.Contains(e.Error(), "finfail"),
		`expected strings.Contains(e.Error(), "finfail"), got:`, e)
}

func TestRunOperationExecRetryError(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{label: "X"}
	o.rurl = "/myresource"
	o.retryMax = 4
	o.exec = func() error {
		return OperationRetryError{
			OriginalError: fmt.Errorf("foobar"),
		}
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return nil
	}
	e := RunOperation(o, app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	// even if rollback fails we expect the error from Exec
	tests.Assert(t, strings.Contains(e.Error(), "foobar"),
		`expected strings.Contains(e.Error(), "foobar"), got:`, e)
	// check that rollback got called
	tests.Assert(t, rollback_cc == 5,
		"expected rollback_cc == 5, got:", rollback_cc)
}

func TestRunOperationExecRetryRollbackFail(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{label: "X"}
	o.rurl = "/myresource"
	o.retryMax = 4
	o.exec = func() error {
		return OperationRetryError{
			OriginalError: fmt.Errorf("foobar"),
		}
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return fmt.Errorf("rollbackfail")
	}
	build_cc := 0
	o.build = func() error {
		build_cc++
		return nil
	}
	e := RunOperation(o, app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	// even if rollback fails we expect the error from Exec
	tests.Assert(t, strings.Contains(e.Error(), "rollbackfail"),
		`expected strings.Contains(e.Error(), "rollbackfail"), got:`, e)
	// check that rollback got called
	tests.Assert(t, rollback_cc == 1,
		"expected rollback_cc == 1, got:", rollback_cc)
	tests.Assert(t, build_cc == 1,
		"expected build_cc == 1, got:", build_cc)
}

func TestRunOperationExecRetryThenBuildFail(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{label: "X"}
	o.rurl = "/myresource"
	o.retryMax = 4
	o.exec = func() error {
		return OperationRetryError{
			OriginalError: fmt.Errorf("foobar"),
		}
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return nil
	}
	build_cc := 0
	o.build = func() error {
		build_cc++
		if build_cc > 1 {
			return fmt.Errorf("buildfail")
		}
		return nil
	}
	e := RunOperation(o, app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	// even if rollback fails we expect the error from Exec
	tests.Assert(t, strings.Contains(e.Error(), "buildfail"),
		`expected strings.Contains(e.Error(), "buildfail"), got:`, e)
	// check that rollback got called
	tests.Assert(t, rollback_cc == 1,
		"expected rollback_cc == 1, got:", rollback_cc)
	tests.Assert(t, build_cc == 2,
		"expected build_cc == 2, got:", build_cc)
}

func TestRunOperationExecRetryThenSucceed(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{label: "X"}
	o.rurl = "/myresource"
	o.retryMax = 4
	exec_cc := 0
	o.exec = func() error {
		exec_cc++
		if exec_cc > 2 {
			return nil
		}
		return OperationRetryError{
			OriginalError: fmt.Errorf("foobar"),
		}
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return nil
	}
	e := RunOperation(o, app.executor)
	// even if rollback fails we expect the error from Exec
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	tests.Assert(t, rollback_cc == 2,
		"expected rollback_cc == 2, got:", rollback_cc)
}

func TestRunOperationExecRetryThenNonRetryError(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{label: "X"}
	o.rurl = "/myresource"
	o.retryMax = 4
	exec_cc := 0
	o.exec = func() error {
		exec_cc++
		if exec_cc > 2 {
			return fmt.Errorf("execfail")
		}
		return OperationRetryError{
			OriginalError: fmt.Errorf("foobar"),
		}
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return nil
	}
	e := RunOperation(o, app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	// even if rollback fails we expect the error from Exec
	tests.Assert(t, strings.Contains(e.Error(), "execfail"),
		`expected strings.Contains(e.Error(), "execfail"), got:`, e)
	// check that rollback got called
	tests.Assert(t, rollback_cc == 3,
		"expected rollback_cc == 3, got:", rollback_cc)
}

func TestExpandSizeFromOp(t *testing.T) {
	op := NewPendingOperationEntry("jjjj")
	op.Actions = append(op.Actions, PendingOperationAction{
		Change: OpExpandVolume,
		Id:     "foofoofoo",
		Delta:  495,
	})
	// this op lacks the expand metadata, should return error
	v, e := expandSizeFromOp(op)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	tests.Assert(t, v == 495, "expected v == 495, got:", v)
}

func TestExpandSizeFromOpErrorHandling(t *testing.T) {
	op := NewPendingOperationEntry("jjjj")
	// this op lacks the expand metadata, should return error
	_, e := expandSizeFromOp(op)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	tests.Assert(t, strings.Contains(e.Error(), "no OpExpandVolume action"),
		`expected strings.Contains(e.Error(), "no OpExpandVolume action"), got:`,
		e)
}

func TestAppServerResetStaleOps(t *testing.T) {
	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)

	// create a app that will only be used to set up the test
	app := NewTestApp(dbfile)
	tests.Assert(t, app != nil)

	// pretend first server startup
	err := app.ServerReset()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		1*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// create pending operations that we will "orphan"
	req := &api.VolumeCreateRequest{}
	req.Size = 1

	vol1 := NewVolumeEntryFromRequest(req)
	vc1 := NewVolumeCreateOperation(vol1, app.db)
	err = vc1.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vol2 := NewVolumeEntryFromRequest(req)
	vc2 := NewVolumeCreateOperation(vol2, app.db)
	err = vc2.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 2, "expected len(pol) == 2, got", len(pol))
		for _, poid := range pol {
			po, e := NewPendingOperationEntryFromId(tx, poid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, po.Status == NewOperation,
				"expected po.Status == NewOperation, got:", po.Status)
		}
		return nil
	})

	// pretend the server was restarted
	err = app.ServerReset()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 2, "expected len(pol) == 2, got", len(pol))
		for _, poid := range pol {
			po, e := NewPendingOperationEntryFromId(tx, poid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, po.Status == StaleOperation,
				"expected po.Status == NewOperation, got:", po.Status)
		}
		return nil
	})
}
