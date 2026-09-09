package portable

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPortableReportBudgetArchiveAndCollision(t *testing.T) {
	root := privateTemp(t)
	a, err := newReports(root, "self-test", BuildInfo{}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	b, err := newReports(root, "self-test", BuildInfo{}, 1<<20)
	if err != nil || a.dir == b.dir {
		t.Fatal("run collision")
	}
	if os.WriteFile(filepath.Join(a.dir, "private-key.pem"), []byte("synthetic-secret"), 0600) != nil {
		t.Fatal("write")
	}
	if a.finish(1, true) != nil || b.finish(3, false) != nil {
		t.Fatal("finalize")
	}
	z, err := zip.OpenReader(filepath.Join(a.dir, "transfer.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, f := range z.File {
		if f.Name == "private-key.pem" {
			t.Fatal("unsafe ZIP member")
		}
	}
	if _, err := os.Stat(filepath.Join(b.dir, "status.complete")); !os.IsNotExist(err) {
		t.Fatal("incomplete marked complete")
	}
	r, _ := newReports(root, "self-test", BuildInfo{}, 1<<20)
	r.used = 1 << 20
	if r.step(Step{}) == nil || r.check(Check{}) == nil {
		t.Fatal("report cap ignored")
	}
	r.file.Close()
	if transferZIP(root, []string{"../secret"}) == nil {
		t.Fatal("unsafe ZIP accepted")
	}
}

func TestPortableReportFinalizationFailure(t *testing.T) {
	r, err := newReports(privateTemp(t), "self-test", BuildInfo{}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	// A preexisting archive cannot be overwritten; no completed marker is valid.
	if err := os.Mkdir(filepath.Join(r.dir, "transfer.zip"), 0700); err != nil {
		t.Fatal(err)
	}
	if r.finish(0, true) == nil {
		t.Fatal("finalization failure ignored")
	}
	if _, err := os.Stat(filepath.Join(r.dir, "status.complete")); !os.IsNotExist(err) {
		t.Fatal("false completed marker")
	}
	data, err := os.ReadFile(filepath.Join(r.dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report Report
	if json.Unmarshal(data, &report) != nil || report.Complete || report.ExitCode != 3 || report.Status != "INCOMPLETE" {
		t.Fatal("false successful report")
	}
	r, err = newReports(privateTemp(t), "self-test", BuildInfo{}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	r.deadline = time.Now().Add(-time.Second)
	if r.finish(0, true) != ErrLimit {
		t.Fatal("shutdown deadline ignored")
	}
	if _, err := os.Stat(filepath.Join(r.dir, "status.complete")); !os.IsNotExist(err) {
		t.Fatal("expired completion")
	}
}
