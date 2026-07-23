package saascompliance

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
)

const complianceEncryptionChunkSize = 64 * 1024

var complianceArtifactMagic = []byte{'M', 'G', 'C', 'E', 1, 0, 0, 0}

func deriveComplianceKey(master []byte) []byte {
	hash := hmac.New(sha256.New, master)
	_, _ = hash.Write([]byte("mochat-go/saas-compliance-export/v1"))
	return hash.Sum(nil)
}

type encryptedWriter struct {
	w      io.Writer
	aead   cipher.AEAD
	buffer []byte
	closed bool
}

func newEncryptedWriter(w io.Writer, key []byte) (*encryptedWriter, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(complianceArtifactMagic); err != nil {
		return nil, err
	}
	return &encryptedWriter{w: w, aead: aead, buffer: make([]byte, 0, complianceEncryptionChunkSize)}, nil
}

func (w *encryptedWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, fmt.Errorf("compliance encryption writer is closed")
	}
	written := 0
	for len(p) > 0 {
		space := complianceEncryptionChunkSize - len(w.buffer)
		if space > len(p) {
			space = len(p)
		}
		w.buffer = append(w.buffer, p[:space]...)
		p = p[space:]
		written += space
		if len(w.buffer) == complianceEncryptionChunkSize {
			if err := w.flush(); err != nil {
				return written, err
			}
		}
	}
	return written, nil
}

func (w *encryptedWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return w.flush()
}

func (w *encryptedWriter) flush() error {
	if len(w.buffer) == 0 {
		return nil
	}
	nonce := make([]byte, w.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := w.aead.Seal(nil, nonce, w.buffer, complianceArtifactMagic)
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

type encryptedReader struct {
	r       *bufio.Reader
	aead    cipher.AEAD
	current []byte
	offset  int
}

func newEncryptedReader(r io.Reader, key []byte) (*encryptedReader, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	buffered := bufio.NewReader(r)
	header := make([]byte, len(complianceArtifactMagic))
	if _, err := io.ReadFull(buffered, header); err != nil {
		return nil, fmt.Errorf("read compliance artifact header: %w", err)
	}
	if !hmac.Equal(header, complianceArtifactMagic) {
		return nil, Invalid("租户数据导出工件格式无效")
	}
	return &encryptedReader{r: buffered, aead: aead}, nil
}

func (r *encryptedReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for r.offset >= len(r.current) {
		nonce := make([]byte, r.aead.NonceSize())
		if _, err := io.ReadFull(r.r, nonce); err != nil {
			if err == io.EOF {
				return 0, io.EOF
			}
			return 0, fmt.Errorf("read compliance artifact nonce: %w", err)
		}
		var length [4]byte
		if _, err := io.ReadFull(r.r, length[:]); err != nil {
			return 0, fmt.Errorf("read compliance artifact chunk length: %w", err)
		}
		chunkLength := binary.BigEndian.Uint32(length[:])
		maxLength := uint32(complianceEncryptionChunkSize + r.aead.Overhead())
		if chunkLength <= uint32(r.aead.Overhead()) || chunkLength > maxLength {
			return 0, Invalid("租户数据导出工件分块长度无效")
		}
		ciphertext := make([]byte, chunkLength)
		if _, err := io.ReadFull(r.r, ciphertext); err != nil {
			return 0, fmt.Errorf("read compliance artifact chunk: %w", err)
		}
		plaintext, err := r.aead.Open(nil, nonce, ciphertext, complianceArtifactMagic)
		if err != nil {
			return 0, Invalid("租户数据导出工件认证失败")
		}
		r.current = plaintext
		r.offset = 0
	}
	n := copy(p, r.current[r.offset:])
	r.offset += n
	return n, nil
}

type exportReadCloser struct {
	reader io.Reader
	closer io.Closer
}

func (r *exportReadCloser) Read(p []byte) (int, error) { return r.reader.Read(p) }
func (r *exportReadCloser) Close() error               { return r.closer.Close() }
