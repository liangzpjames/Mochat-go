package saasbackup

import (
	"bufio"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const backupEncryptionChunkSize = 64 * 1024

var backupArtifactMagic = []byte{'M', 'G', 'B', 'K', 1, 0, 0, 0}
var encryptionKeyIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func ParseEncryptionKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if len(value) == 64 {
		if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	encodings := []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(value)
		if err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("backup encryption key must be 32 bytes encoded as base64 or 64 hex characters")
}

func ParseEncryptionKeyRing(value string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	value = strings.TrimSpace(value)
	if value == "" {
		return result, nil
	}
	var encoded map[string]string
	if err := json.Unmarshal([]byte(value), &encoded); err != nil {
		return nil, fmt.Errorf("backup encryption keys must be a JSON object: %w", err)
	}
	if len(encoded) == 0 {
		return nil, fmt.Errorf("backup encryption keys JSON object must not be empty")
	}
	for id, raw := range encoded {
		id = strings.TrimSpace(id)
		if !encryptionKeyIDPattern.MatchString(id) {
			return nil, fmt.Errorf("backup encryption key id %q is invalid", id)
		}
		key, err := ParseEncryptionKey(raw)
		if err != nil {
			return nil, fmt.Errorf("parse backup encryption key %q: %w", id, err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("backup encryption key %q is empty", id)
		}
		result[id] = key
	}
	return result, nil
}

func safeArtifactPath(root, name string) (string, error) {
	root = strings.TrimSpace(root)
	name = strings.TrimSpace(name)
	if root == "" || name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return "", Invalid("备份工件路径无效")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(absRoot, name), nil
}

type chunkEncryptWriter struct {
	w      io.Writer
	aead   cipher.AEAD
	buffer []byte
	closed bool
}

func newChunkEncryptWriter(w io.Writer, key []byte) (*chunkEncryptWriter, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(backupArtifactMagic); err != nil {
		return nil, err
	}
	return &chunkEncryptWriter{w: w, aead: aead, buffer: make([]byte, 0, backupEncryptionChunkSize)}, nil
}

func (w *chunkEncryptWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, fmt.Errorf("backup encryption writer is closed")
	}
	written := 0
	for len(p) > 0 {
		space := backupEncryptionChunkSize - len(w.buffer)
		if space > len(p) {
			space = len(p)
		}
		w.buffer = append(w.buffer, p[:space]...)
		p = p[space:]
		written += space
		if len(w.buffer) == backupEncryptionChunkSize {
			if err := w.flush(); err != nil {
				return written, err
			}
		}
	}
	return written, nil
}

func (w *chunkEncryptWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return w.flush()
}

func (w *chunkEncryptWriter) flush() error {
	if len(w.buffer) == 0 {
		return nil
	}
	nonce := make([]byte, w.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := w.aead.Seal(nil, nonce, w.buffer, backupArtifactMagic)
	if _, err := w.w.Write(nonce); err != nil {
		return err
	}
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(ciphertext)))
	if _, err := w.w.Write(length[:]); err != nil {
		return err
	}
	if _, err := w.w.Write(ciphertext); err != nil {
		return err
	}
	w.buffer = w.buffer[:0]
	return nil
}

type chunkDecryptReader struct {
	r       *bufio.Reader
	aead    cipher.AEAD
	current []byte
	offset  int
}

func newChunkDecryptReader(r io.Reader, key []byte) (*chunkDecryptReader, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(r)
	magic := make([]byte, len(backupArtifactMagic))
	if _, err := io.ReadFull(reader, magic); err != nil {
		return nil, fmt.Errorf("read backup artifact header: %w", err)
	}
	if string(magic) != string(backupArtifactMagic) {
		return nil, fmt.Errorf("backup artifact encryption header is invalid")
	}
	return &chunkDecryptReader{r: reader, aead: aead}, nil
}

func (r *chunkDecryptReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.offset >= len(r.current) {
		if err := r.readChunk(); err != nil {
			return 0, err
		}
	}
	n := copy(p, r.current[r.offset:])
	r.offset += n
	return n, nil
}

func (r *chunkDecryptReader) readChunk() error {
	nonce := make([]byte, r.aead.NonceSize())
	if _, err := io.ReadFull(r.r, nonce); err != nil {
		if err == io.EOF {
			return io.EOF
		}
		return fmt.Errorf("read backup artifact nonce: %w", err)
	}
	var length [4]byte
	if _, err := io.ReadFull(r.r, length[:]); err != nil {
		return fmt.Errorf("read backup artifact chunk length: %w", err)
	}
	ciphertextLength := int(binary.BigEndian.Uint32(length[:]))
	if ciphertextLength <= r.aead.Overhead() || ciphertextLength > backupEncryptionChunkSize+r.aead.Overhead() {
		return fmt.Errorf("backup artifact chunk length is invalid")
	}
	ciphertext := make([]byte, ciphertextLength)
	if _, err := io.ReadFull(r.r, ciphertext); err != nil {
		return fmt.Errorf("read backup artifact chunk: %w", err)
	}
	plaintext, err := r.aead.Open(nil, nonce, ciphertext, backupArtifactMagic)
	if err != nil {
		return fmt.Errorf("decrypt backup artifact chunk: %w", err)
	}
	r.current = plaintext
	r.offset = 0
	return nil
}

type artifactSQLReadCloser struct {
	gzip *gzip.Reader
	file *os.File
}

func (r *artifactSQLReadCloser) Read(p []byte) (int, error) { return r.gzip.Read(p) }

func (r *artifactSQLReadCloser) Close() error {
	gzipErr := r.gzip.Close()
	fileErr := r.file.Close()
	if gzipErr != nil {
		return gzipErr
	}
	return fileErr
}

func openArtifactSQL(path string, encrypted bool, key []byte) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var compressed io.Reader = file
	if encrypted {
		if len(key) != 32 {
			file.Close()
			return nil, Unavailable("备份加密密钥未配置或长度无效")
		}
		compressed, err = newChunkDecryptReader(file, key)
		if err != nil {
			file.Close()
			return nil, err
		}
	}
	gzipReader, err := gzip.NewReader(compressed)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("open backup gzip stream: %w", err)
	}
	return &artifactSQLReadCloser{gzip: gzipReader, file: file}, nil
}

type hashCountingWriter struct {
	w     io.Writer
	hash  hash.Hash
	count int64
}

func newHashCountingWriter(w io.Writer) *hashCountingWriter {
	return &hashCountingWriter{w: w, hash: sha256.New()}
}

func (w *hashCountingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if n > 0 {
		_, _ = w.hash.Write(p[:n])
		w.count += int64(n)
	}
	return n, err
}

func (w *hashCountingWriter) Sum() string { return hex.EncodeToString(w.hash.Sum(nil)) }

func hashArtifact(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	h := sha256.New()
	size, err := io.Copy(h, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}
