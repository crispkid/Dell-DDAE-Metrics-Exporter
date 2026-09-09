package portable

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
)

type Exchange = ddae.DiagnosticExchange

const captureMagic = "DDAECAP1"
const maxRecordBytes = 96 << 20
const finalReserve = 4096

type captureHeader struct {
	Version int    `json:"version"`
	Run     string `json:"run"`
	Wrapped []byte `json:"wrapped_key"`
}
type captureRecord struct {
	Sequence uint64    `json:"sequence"`
	Exchange *Exchange `json:"exchange,omitempty"`
	Final    bool      `json:"final"`
	Complete bool      `json:"complete"`
	Count    uint64    `json:"count"`
}
type CaptureWriter struct {
	file           *os.File
	aead           cipher.AEAD
	hash           [32]byte
	sequence       uint64
	used, max      int64
	closed, failed bool
}

func ReadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := readBounded(path, 64<<10)
	if err != nil {
		return nil, ErrInput
	}
	block, rest := pem.Decode(data)
	if block == nil || len(rest) != 0 || block.Type != "PUBLIC KEY" {
		return nil, ErrInput
	}
	value, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, ErrInput
	}
	key, ok := value.(*rsa.PublicKey)
	if !ok || key.N.BitLen() < 3072 || key.N.BitLen() > 8192 {
		return nil, ErrInput
	}
	return key, nil
}
func ReadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := readBounded(path, 64<<10)
	if err != nil {
		return nil, ErrInput
	}
	block, rest := pem.Decode(data)
	if block == nil || len(rest) != 0 || block.Type != "PRIVATE KEY" {
		return nil, ErrInput
	}
	value, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, ErrInput
	}
	key, ok := value.(*rsa.PrivateKey)
	if !ok || key.N.BitLen() < 3072 || key.N.BitLen() > 8192 || key.Validate() != nil {
		return nil, ErrInput
	}
	return key, nil
}
func GenerateKeys(privatePath, publicPath string) error {
	if privatePath == publicPath {
		return ErrInput
	}
	for _, p := range []string{privatePath, publicPath} {
		if safePath(p) != nil {
			return ErrInput
		}
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			return ErrInput
		}
	}
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return ErrInput
	}
	priv, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return ErrInput
	}
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return ErrInput
	}
	for _, p := range []string{privatePath, publicPath} {
		if privateDir(filepath.Dir(p)) != nil {
			return ErrStorage
		}
	}
	if err := privateWrite(privatePath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: priv})); err != nil {
		return err
	}
	return privateWrite(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}))
}
func NewCapture(path string, key *rsa.PublicKey, limit int64) (*CaptureWriter, error) {
	if key == nil || key.N.BitLen() < 3072 || key.N.BitLen() > 8192 || limit < 1<<20 || limit > 2<<30 {
		return nil, ErrInput
	}
	secret := make([]byte, 32)
	run := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		return nil, ErrStorage
	}
	if _, err := rand.Read(run); err != nil {
		return nil, ErrStorage
	}
	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, key, secret, []byte(captureMagic))
	if err != nil {
		return nil, ErrInput
	}
	header, err := json.Marshal(captureHeader{Version: 1, Run: hex.EncodeToString(run), Wrapped: wrapped})
	if err != nil {
		return nil, ErrStorage
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, ErrStorage
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrStorage
	}
	f, err := privateCreate(path)
	if err != nil {
		return nil, err
	}
	w := &CaptureWriter{file: f, aead: aead, hash: sha256.Sum256(header), max: limit}
	if w.write([]byte(captureMagic)) != nil || w.frame(header) != nil {
		f.Close()
		return nil, ErrStorage
	}
	return w, nil
}
func (w *CaptureWriter) write(data []byte) error {
	n, err := w.file.Write(data)
	w.used += int64(n)
	if err != nil || n != len(data) {
		w.failed = true
		return ErrStorage
	}
	return nil
}
func (w *CaptureWriter) frame(data []byte) error {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(data)))
	if w.write(size[:]) != nil {
		return ErrStorage
	}
	return w.write(data)
}
func recordNonce(seq uint64) []byte {
	nonce := make([]byte, 12)
	binary.BigEndian.PutUint64(nonce[4:], seq)
	return nonce
}
func recordAAD(hash [32]byte, seq uint64) []byte {
	aad := make([]byte, 40)
	copy(aad, hash[:])
	binary.BigEndian.PutUint64(aad[32:], seq)
	return aad
}
func (w *CaptureWriter) record(r captureRecord, final bool) error {
	if w.closed || w.failed || w.sequence >= 10001 {
		return ErrStorage
	}
	data, err := json.Marshal(r)
	if err != nil || len(data) > maxRecordBytes-w.aead.Overhead() {
		return ErrLimit
	}
	reserve := int64(finalReserve)
	if final {
		reserve = 0
	}
	if w.used+int64(len(data)+w.aead.Overhead()+4)+reserve > w.max {
		return ErrLimit
	}
	ciphertext := w.aead.Seal(nil, recordNonce(w.sequence), data, recordAAD(w.hash, w.sequence))
	if w.frame(ciphertext) != nil {
		return ErrStorage
	}
	if w.file.Sync() != nil {
		w.failed = true
		return ErrStorage
	}
	w.sequence++
	return nil
}
func (w *CaptureWriter) Append(e Exchange) error {
	if e.Operation == "token" {
		return ErrInput
	}
	return w.record(captureRecord{Sequence: w.sequence, Exchange: &e}, false)
}
func (w *CaptureWriter) Finish(complete bool) error {
	if w.closed {
		return ErrStorage
	}
	err := w.record(captureRecord{Sequence: w.sequence, Final: true, Complete: complete, Count: w.sequence}, true)
	w.closed = true
	closeErr := w.file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return ErrStorage
	}
	return nil
}
func readFrame(r io.Reader, max uint32) ([]byte, error) {
	var length [4]byte
	if _, err := io.ReadFull(r, length[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(length[:])
	if n == 0 || n > max {
		return nil, ErrIntegrity
	}
	b := make([]byte, int(n))
	_, err := io.ReadFull(r, b)
	return b, err
}

// ReadCapture authenticates records before exposing them. An incomplete tail
// returns complete=false plus an error; already visited records were authentic.
func ReadCapture(path string, key *rsa.PrivateKey, visit func(Exchange) error) (bool, error) {
	if key == nil || key.N.BitLen() < 3072 || key.N.BitLen() > 8192 || key.Validate() != nil || safePath(path) != nil {
		return false, ErrInput
	}
	f, err := os.Open(path)
	if err != nil {
		return false, ErrInput
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() > 2<<30 {
		return false, ErrInput
	}
	r := bufio.NewReader(io.LimitReader(f, 2<<30))
	magic := make([]byte, len(captureMagic))
	if _, err := io.ReadFull(r, magic); err != nil || string(magic) != captureMagic {
		return false, ErrIntegrity
	}
	h, err := readFrame(r, 16<<10)
	if err != nil {
		return false, ErrIntegrity
	}
	var header captureHeader
	if json.Unmarshal(h, &header) != nil || header.Version != 1 || len(header.Run) != 32 {
		return false, ErrIntegrity
	}
	secret, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, key, header.Wrapped, []byte(captureMagic))
	if err != nil || len(secret) != 32 {
		return false, ErrIntegrity
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		return false, ErrIntegrity
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return false, ErrIntegrity
	}
	hash := sha256.Sum256(h)
	for seq := uint64(0); seq <= 10000; seq++ {
		ciphertext, err := readFrame(r, maxRecordBytes)
		if err != nil {
			return false, ErrIntegrity
		}
		data, err := aead.Open(nil, recordNonce(seq), ciphertext, recordAAD(hash, seq))
		if err != nil {
			return false, ErrIntegrity
		}
		var record captureRecord
		if json.Unmarshal(data, &record) != nil || record.Sequence != seq {
			return false, ErrIntegrity
		}
		if record.Final {
			if record.Exchange != nil || record.Count != seq {
				return false, ErrIntegrity
			}
			if _, err := r.ReadByte(); err != io.EOF {
				return false, ErrIntegrity
			}
			return record.Complete, nil
		}
		if record.Exchange == nil || !businessOperation(record.Exchange.Operation) || len(record.Exchange.Body) > 64<<20 {
			return false, ErrIntegrity
		}
		if err := visit(*record.Exchange); err != nil {
			return false, err
		}
	}
	return false, ErrIntegrity
}

func businessOperation(op string) bool {
	for _, entry := range ddae.ApprovedOperations() {
		if entry.Collector == op {
			return true
		}
	}
	return false
}
