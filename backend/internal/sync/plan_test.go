package sync

import (
	"testing"
	"time"

	"github.com/goodylili/dropboy/internal/store"
)

func TestPlanUploadsNewLocalFile(t *testing.T) {
	local := map[string]localEntry{
		"/x/a.txt": {Path: "/x/a.txt", Hash: "h1"},
	}
	ops := plan(local, nil, nil, "m1")
	if len(ops) != 1 || ops[0].Kind != OpUpload {
		t.Fatalf("want one upload op, got %+v", ops)
	}
}

func TestPlanDownloadsNewRemoteFile(t *testing.T) {
	remote := map[string]remoteEntry{
		"/x/b.txt": {Key: "dropboy/v1/m1/x/b.txt", Path: "/x/b.txt", Hash: "h2"},
	}
	ops := plan(nil, remote, nil, "m1")
	if len(ops) != 1 || ops[0].Kind != OpDownload {
		t.Fatalf("want one download op, got %+v", ops)
	}
}

func TestPlanDetectsConflict(t *testing.T) {
	local := map[string]localEntry{"/x/c.txt": {Path: "/x/c.txt", Hash: "local"}}
	remote := map[string]remoteEntry{"/x/c.txt": {Key: "k", Path: "/x/c.txt", Hash: "remote"}}
	state := map[string]store.Entry{"/x/c.txt": {Path: "/x/c.txt", LocalHash: "original", LastSyncedAt: time.Now()}}
	ops := plan(local, remote, state, "m1")
	if len(ops) != 1 || ops[0].Kind != OpConflict {
		t.Fatalf("want one conflict op, got %+v", ops)
	}
}

func TestPlanLocalChangedOnlyUploads(t *testing.T) {
	local := map[string]localEntry{"/x/d.txt": {Path: "/x/d.txt", Hash: "new"}}
	remote := map[string]remoteEntry{"/x/d.txt": {Key: "k", Path: "/x/d.txt", Hash: "old"}}
	state := map[string]store.Entry{"/x/d.txt": {Path: "/x/d.txt", LocalHash: "old"}}
	ops := plan(local, remote, state, "m1")
	if len(ops) != 1 || ops[0].Kind != OpUpload {
		t.Fatalf("want upload, got %+v", ops)
	}
}

func TestPlanRemoteChangedOnlyDownloads(t *testing.T) {
	local := map[string]localEntry{"/x/e.txt": {Path: "/x/e.txt", Hash: "old"}}
	remote := map[string]remoteEntry{"/x/e.txt": {Key: "k", Path: "/x/e.txt", Hash: "new"}}
	state := map[string]store.Entry{"/x/e.txt": {Path: "/x/e.txt", LocalHash: "old"}}
	ops := plan(local, remote, state, "m1")
	if len(ops) != 1 || ops[0].Kind != OpDownload {
		t.Fatalf("want download, got %+v", ops)
	}
}

func TestPlanLocallyDeletedRemovesRemote(t *testing.T) {
	remote := map[string]remoteEntry{"/x/f.txt": {Key: "k", Path: "/x/f.txt", Hash: "h"}}
	state := map[string]store.Entry{"/x/f.txt": {Path: "/x/f.txt", LocalHash: "h"}}
	ops := plan(nil, remote, state, "m1")
	if len(ops) != 1 || ops[0].Kind != OpDeleteRemote {
		t.Fatalf("want delete-remote, got %+v", ops)
	}
}

func TestKeyForRoundTrip(t *testing.T) {
	key := keyFor("m1", "/foo/bar.txt")
	got := pathFromKey(key, "m1")
	if got != "/foo/bar.txt" {
		t.Errorf("round trip lost path: %q", got)
	}
}
