package portable

import (
	"archive/zip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type BuildInfo struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
	Source   string `json:"source_sha256"`
}
type Step struct {
	RequestID       int          `json:"request_id"`
	Operation       string       `json:"operation"`
	Started         string       `json:"started_utc"`
	Milliseconds    int64        `json:"duration_ms"`
	HTTPStatus      int          `json:"http_status"`
	HTTP            bool         `json:"http_ok"`
	CaptureComplete bool         `json:"capture_complete"`
	Reason          string       `json:"reason,omitempty"`
	Parse           *ParseResult `json:"parser,omitempty"`
}
type Check struct {
	Operation                       string `json:"operation"`
	Status                          string `json:"status"`
	Reason                          string `json:"reason,omitempty"`
	Available, Selected, Successful int
}
type Report struct {
	Schema                      int       `json:"schema_version"`
	RunID                       string    `json:"run_id"`
	Mode                        string    `json:"mode"`
	OS                          string    `json:"os"`
	Architecture                string    `json:"architecture"`
	Build                       BuildInfo `json:"build"`
	Started                     string    `json:"started_utc"`
	Ended                       string    `json:"ended_utc"`
	Status                      string    `json:"status"`
	ExitCode                    int       `json:"exit_code"`
	Complete                    bool      `json:"complete"`
	InsecureTLS                 bool      `json:"insecure_tls"`
	NativeWindowsValidated      bool      `json:"native_windows_validated"`
	FieldCompatibilityValidated bool      `json:"field_compatibility_validated"`
	Steps                       []Step    `json:"steps"`
	Checks                      []Check   `json:"checks"`
}
type reports struct {
	dir       string
	file      *os.File
	used, max int64
	written   int64
	deadline  time.Time
	report    Report
}

func newReports(root, mode string, build BuildInfo, maxBytes int64) (*reports, error) {
	if maxBytes < 1<<20 || maxBytes > 64<<20 || privateDir(root) != nil {
		return nil, ErrStorage
	}
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return nil, ErrStorage
	}
	id := time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(random)
	dir := filepath.Join(root, id)
	if os.Mkdir(dir, 0700) != nil || protect(dir, true) != nil {
		return nil, ErrStorage
	}
	file, err := privateCreate(filepath.Join(dir, "steps.jsonl"))
	if err != nil {
		return nil, err
	}
	r := &reports{dir: dir, file: file, max: maxBytes, report: Report{Schema: 1, RunID: id, Mode: mode, OS: runtime.GOOS, Architecture: runtime.GOARCH, Build: build, Started: time.Now().UTC().Format(time.RFC3339Nano), Steps: []Step{}, Checks: []Check{}}}
	return r, nil
}
func (r *reports) step(s Step) error {
	data, err := json.Marshal(s)
	if err != nil {
		return ErrStorage
	}
	// Charge for both streamed and final copies and reserve space for finalization.
	if r.used+int64(len(data)+1)*4+256<<10 > r.max {
		return ErrLimit
	}
	if _, err := r.file.Write(append(data, '\n')); err != nil {
		return ErrStorage
	}
	if r.file.Sync() != nil {
		return ErrStorage
	}
	r.used += int64(len(data)+1) * 4
	r.written += int64(len(data) + 1)
	r.report.Steps = append(r.report.Steps, s)
	return nil
}
func (r *reports) check(c Check) error {
	if r.used+2048+256<<10 > r.max {
		return ErrLimit
	}
	r.used += 2048
	r.report.Checks = append(r.report.Checks, c)
	return nil
}
func (r *reports) finish(code int, complete bool) (result error) {
	reportWritten := false
	defer func() {
		if result == nil || !reportWritten {
			return
		}
		// Best effort only: a failed disk may also reject this status correction.
		// Absence of status.complete remains the definitive finalization signal.
		r.report.Complete = false
		r.report.Status = "INCOMPLETE"
		if r.report.ExitCode != 130 {
			r.report.ExitCode = 3
		}
		data, _ := json.Marshal(r.report)
		path := filepath.Join(r.dir, "report.json")
		if safePath(path) == nil {
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0600)
			if err == nil {
				_, _ = f.Write(data)
				_ = f.Sync()
				_ = f.Close()
			}
		}
	}()
	closeErr := r.file.Close()
	if closeErr != nil {
		return ErrStorage
	}
	r.report.Ended = time.Now().UTC().Format(time.RFC3339Nano)
	r.report.ExitCode = code
	r.report.Complete = complete
	r.report.Status = "PASS"
	if code == 1 || code == 2 {
		r.report.Status = "FAIL"
	} else if code != 0 || !complete {
		r.report.Status = "INCOMPLETE"
	}
	data, err := json.Marshal(r.report)
	if err != nil {
		return ErrStorage
	}
	summary := fmt.Sprintf("DDAE 診斷結果: %s\n模式: %s\n執行編號: %s\n退出代碼: %d\n完整執行: %t\nTLS 驗證略過: %t\n\n", r.report.Status, r.report.Mode, r.report.RunID, code, complete, r.report.InsecureTLS)
	for _, c := range r.report.Checks {
		summary += fmt.Sprintf("%s: %s (%s) available=%d selected=%d successful=%d\n", c.Operation, c.Status, c.Reason, c.Available, c.Selected, c.Successful)
	}
	summary += "\nHTTP 成功、Parser 成功、資料完整性與明細覆蓋率分開記錄。\n401/403: 檢查認證/授權；404: 檢查 prefix；decode/validation: 檢查加密回應與規格。\n此報告不是獨立 Windows／DDAE 相容性認證。分享前請依組織資料政策檢查。\n"
	shapes := map[string]*ParseResult{}
	for _, s := range r.report.Steps {
		if s.Parse != nil {
			shapes[s.Operation] = s.Parse
		}
	}
	shapeJSON, err := json.Marshal(shapes)
	if err != nil {
		return ErrStorage
	}
	outputs := map[string][]byte{"report.json": data, "summary.txt": []byte(summary), "schema-observations.json": shapeJSON, "diagnostic.log": []byte("diagnostic_started\ndiagnostic_finished status=" + r.report.Status + "\n")}
	total := r.written
	for _, b := range outputs {
		total += int64(len(b))
	}
	if total > r.max {
		return ErrLimit
	}
	for name, b := range outputs {
		if r.expired() {
			return ErrLimit
		}
		if err := privateWrite(filepath.Join(r.dir, name), b); err != nil {
			return err
		}
		if name == "report.json" {
			reportWritten = true
		}
	}
	members := []string{"summary.txt", "report.json", "steps.jsonl", "schema-observations.json", "diagnostic.log"}
	if _, err := os.Stat(filepath.Join(r.dir, "http-capture.ddaecap")); err == nil {
		members = append(members, "http-capture.ddaecap")
	}
	sums := map[string]string{}
	for _, name := range members {
		if r.expired() {
			return ErrLimit
		}
		hash, err := fileHash(filepath.Join(r.dir, name))
		if err != nil {
			return err
		}
		sums[name] = hash
	}
	manifest, err := json.Marshal(struct {
		Build BuildInfo         `json:"build"`
		Files map[string]string `json:"files"`
	}{r.report.Build, sums})
	if err != nil {
		return ErrStorage
	}
	if err := privateWrite(filepath.Join(r.dir, "manifest.json"), manifest); err != nil {
		return err
	}
	members = append(members, "manifest.json")
	if complete {
		members = append(members, "status.complete")
	}
	if err := transferZIPUntil(r.dir, members, r.deadline); err != nil {
		return err
	}
	if r.expired() {
		return ErrLimit
	}
	if complete {
		return privateWrite(filepath.Join(r.dir, "status.complete"), []byte("complete\n"))
	}
	return nil
}
func (r *reports) expired() bool { return !r.deadline.IsZero() && time.Now().After(r.deadline) }
func fileHash(path string) (string, error) {
	if safePath(path) != nil {
		return "", ErrInput
	}
	f, err := os.Open(path)
	if err != nil {
		return "", ErrStorage
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", ErrStorage
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func transferZIP(dir string, members []string) error {
	return transferZIPUntil(dir, members, time.Time{})
}
func transferZIPUntil(dir string, members []string, deadline time.Time) error {
	f, err := privateCreate(filepath.Join(dir, "transfer.zip"))
	if err != nil {
		return err
	}
	z := zip.NewWriter(f)
	failed := false
	for _, name := range members {
		if !deadline.IsZero() && time.Now().After(deadline) {
			failed = true
			break
		}
		switch name {
		case "summary.txt", "report.json", "steps.jsonl", "schema-observations.json", "diagnostic.log", "manifest.json", "http-capture.ddaecap", "status.complete":
		default:
			failed = true
			continue
		}
		if name == "status.complete" {
			entry, err := z.Create(name)
			if err == nil {
				_, err = io.WriteString(entry, "complete\n")
			}
			if err != nil {
				failed = true
				break
			}
			continue
		}
		src, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			failed = true
			break
		}
		entry, err := z.Create(name)
		if err == nil {
			_, err = io.Copy(entry, deadlineReader{Reader: src, deadline: deadline})
		}
		src.Close()
		if err != nil {
			failed = true
			break
		}
	}
	e1 := z.Close()
	e2 := f.Close()
	if failed || e1 != nil || e2 != nil {
		return ErrStorage
	}
	return nil
}

type deadlineReader struct {
	io.Reader
	deadline time.Time
}

func (r deadlineReader) Read(p []byte) (int, error) {
	if !r.deadline.IsZero() && time.Now().After(r.deadline) {
		return 0, ErrLimit
	}
	return r.Reader.Read(p)
}
