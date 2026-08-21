// Command seed_dev_wecom_recordings creates local-only WAV fixtures and marks
// the three sample audio rows as enterprise-WeChat synchronized recordings.
// It is intentionally explicit (never called by application startup).
package main

import (
	"database/sql"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
)

type fixture struct {
	id       int64
	name     string
	path     string
	message  string
	sender   string
	receiver string
	seconds  int
	syncedAt string
}

func main() {
	dsn := flag.String("dsn", os.Getenv("MOCHAT_MYSQL_DSN"), "MySQL DSN")
	out := flag.String("output", filepath.Join("tmp", "dev-wecom-recordings"), "host directory for generated files")
	flag.Parse()
	if *dsn == "" {
		*dsn = "mochat:mochat_pass@tcp(127.0.0.1:13316)/mochat?parseTime=true&loc=Local"
	}
	fixtures := []fixture{
		{1, "客户咨询录音-001.wav", "audio/sim/2026/08/customer-consult-001.wav", "wecom-msg-dev-001", "张伟", "陈经理", 32, "2026-08-20 10:20:00"},
		{2, "客户咨询录音-002.wav", "audio/sim/2026/08/customer-consult-002.wav", "wecom-msg-dev-002", "陈经理", "张伟", 48, "2026-08-19 15:40:00"},
		{3, "售后回访录音-001.wav", "audio/sim/2026/08/aftersale-001.wav", "wecom-msg-dev-003", "王芳", "李总", 40, "2026-08-18 09:15:00"},
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		panic(err)
	}
	for _, item := range fixtures {
		path := filepath.Join(*out, filepath.FromSlash(item.path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			panic(err)
		}
		if err := os.WriteFile(path, wav(item.seconds), 0o644); err != nil {
			panic(err)
		}
	}

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		panic(err)
	}
	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	for _, item := range fixtures {
		_, err = tx.Exec(`
			UPDATE mochat_go_audio_objects
			SET original_name = ?, source = 'wecom_sync', message_id = ?, sender_name = ?, receiver_name = ?,
				relative_path = ?, content_type = 'audio/wav', size_bytes = ?, duration_seconds = ?, synced_at = ?, deleted_at = NULL, deleted_by = 0, updated_at = NOW()
			WHERE id = ?`, item.name, item.message, item.sender, item.receiver, item.path, int64(len(wav(item.seconds))), item.seconds, item.syncedAt, item.id)
		if err != nil {
			_ = tx.Rollback()
			panic(err)
		}
	}
	if err := tx.Commit(); err != nil {
		panic(err)
	}
	fmt.Printf("seeded %d development recordings in %s\n", len(fixtures), *out)
}

func wav(seconds int) []byte {
	const sampleRate = 8000
	const channels = 1
	const bits = 16
	dataSize := sampleRate * channels * bits / 8 * seconds
	payload := make([]byte, 44+dataSize)
	copy(payload[0:4], "RIFF")
	binary.LittleEndian.PutUint32(payload[4:8], uint32(len(payload)-8))
	copy(payload[8:12], "WAVE")
	copy(payload[12:16], "fmt ")
	binary.LittleEndian.PutUint32(payload[16:20], 16)
	binary.LittleEndian.PutUint16(payload[20:22], 1)
	binary.LittleEndian.PutUint16(payload[22:24], channels)
	binary.LittleEndian.PutUint32(payload[24:28], sampleRate)
	binary.LittleEndian.PutUint32(payload[28:32], sampleRate*channels*bits/8)
	binary.LittleEndian.PutUint16(payload[32:34], channels*bits/8)
	binary.LittleEndian.PutUint16(payload[34:36], bits)
	copy(payload[36:40], "data")
	binary.LittleEndian.PutUint32(payload[40:44], uint32(dataSize))
	return payload
}
