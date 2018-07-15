package glusterfs

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"net/http"
)

func (a *App) SnapshotInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	var info *api.SnapshotInfoResponse
	err := a.db.View(func(tx *bolt.Tx) error {
		entry, err := NewSnapshotEntryFromId(tx, id)
		if err == ErrNotFound || !entry.Visible() {
			http.Error(w, "Id not found", http.StatusNotFound)
			return ErrNotFound
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}
		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusExpectationFailed)
			return err
		}
		return nil
	})
	if err != nil {
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		panic(err)
	}
}

func (a *App) SnapshotList(w http.ResponseWriter, r *http.Request) {
	var list api.SnapshotListResponse
	err := a.db.View(func(tx *bolt.Tx) error {
		var err error
		list.Snapshots, err = ListCompleteSnapshots(tx)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Err(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Send list back
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(list); err != nil {
		panic(err)
	}
}

func (a *App) SnapshotDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	var snapshotEntry *SnapshotEntry
	err := a.db.View(func(tx *bolt.Tx) error {
		var err error
		snapshotEntry, err = NewSnapshotEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}
		return nil
	})
	if err != nil {
		return
	}

	sdel := NewSnapshotDeleteOperation(snapshotEntry, a.db)
	if err := AsyncHttpOperation(a, w, r, sdel); err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to delete snapshot: %v", err),
			http.StatusInternalServerError)
		return
	}
}
