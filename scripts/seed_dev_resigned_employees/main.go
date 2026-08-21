package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const fixturePrefix = "MOCHAT-DEV-RESIGNED"

type employeeFixture struct {
	key          string
	name         string
	department   string
	contactID    int
	roomID       int
	firstMessage string
	secondMsg    string
}

var fixtures = []employeeFixture{
	{key: "001", name: "周岚（已离职）", department: "销售部", contactID: 2001, roomID: 3001, firstMessage: "客户报价跟进已完成，请接续处理。", secondMsg: "交接资料已同步到客户档案。"},
	{key: "002", name: "高远（已离职）", department: "客服部", contactID: 2002, roomID: 3003, firstMessage: "客户售后问题已记录，等待回访。", secondMsg: "请接手后继续跟进服务进度。"},
	{key: "003", name: "沈宁（已离职）", department: "销售部", contactID: 2003, roomID: 3002, firstMessage: "客户群交接事项已整理。", secondMsg: "新的负责人可以从这里继续查看历史消息。"},
}

func main() {
	dsn := flag.String("dsn", "mochat:mochat_pass@tcp(127.0.0.1:13316)/mochat?parseTime=true&loc=Local", "MariaDB DSN")
	corpID := flag.Int("corp-id", 1, "development corp id")
	cleanup := flag.Bool("cleanup", false, "remove this fixture instead of seeding it")
	flag.Parse()

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	if *cleanup {
		err = removeFixture(ctx, tx, *corpID)
	} else {
		err = seedFixture(ctx, tx, *corpID)
	}
	if err != nil {
		_ = tx.Rollback()
		log.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	if *cleanup {
		fmt.Printf("removed %d resigned employee fixtures for corp %d\n", len(fixtures), *corpID)
	} else {
		fmt.Printf("seeded %d resigned employee fixtures for corp %d\n", len(fixtures), *corpID)
	}
}

func departmentID(ctx context.Context, tx *sql.Tx, corpID int, name string) (int, error) {
	var id int
	err := tx.QueryRowContext(ctx, `SELECT id FROM mc_work_department WHERE corp_id=? AND name=? AND deleted_at IS NULL ORDER BY id LIMIT 1`, corpID, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("find department %q: %w", name, err)
	}
	return id, nil
}

func employeeID(ctx context.Context, tx *sql.Tx, corpID int, key string) (int, error) {
	var id int
	err := tx.QueryRowContext(ctx, `SELECT id FROM mc_work_employee WHERE corp_id=? AND wx_user_id=? ORDER BY id DESC LIMIT 1`, corpID, fixturePrefix+"-"+key).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func seedFixture(ctx context.Context, tx *sql.Tx, corpID int) error {
	for index, fixture := range fixtures {
		department, err := departmentID(ctx, tx, corpID, fixture.department)
		if err != nil {
			return err
		}
		id, err := employeeID(ctx, tx, corpID, fixture.key)
		if err == sql.ErrNoRows {
			result, insertErr := tx.ExecContext(ctx, `
				INSERT INTO mc_work_employee (wx_user_id, corp_id, name, position, status, wx_main_department_id, main_department_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, 5, ?, ?, NOW(), NOW())
			`, fixturePrefix+"-"+fixture.key, corpID, fixture.name, "开发验收离职员工", department, department)
			if insertErr != nil {
				return fmt.Errorf("insert employee %s: %w", fixture.key, insertErr)
			}
			newID, idErr := result.LastInsertId()
			if idErr != nil {
				return idErr
			}
			id = int(newID)
		} else if err != nil {
			return err
		} else {
			if _, err := tx.ExecContext(ctx, `UPDATE mc_work_employee SET name=?, position=?, status=5, wx_main_department_id=?, main_department_id=?, deleted_at=NULL, updated_at=NOW() WHERE id=? AND corp_id=?`, fixture.name, "开发验收离职员工", department, department, id, corpID); err != nil {
				return fmt.Errorf("update employee %s: %w", fixture.key, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mc_work_employee_department (employee_id, department_id, created_at, updated_at)
			SELECT ?, ?, NOW(), NOW() FROM DUAL
			WHERE NOT EXISTS (SELECT 1 FROM mc_work_employee_department WHERE employee_id=? AND department_id=? AND deleted_at IS NULL)
		`, id, department, id, department); err != nil {
			return fmt.Errorf("link employee %s to department: %w", fixture.key, err)
		}
		if err := upsertMessage(ctx, tx, corpID, id, fixture, index, false); err != nil {
			return err
		}
	}
	return nil
}

func upsertMessage(ctx context.Context, tx *sql.Tx, corpID, employeeID int, fixture employeeFixture, index int, cleanup bool) error {
	baseSeq := int64(9700000000000000 + index*10)
	messages := []struct {
		key    string
		toType int
		toID   int
		roomID int
		text   string
		sentAt string
	}{
		{key: "customer", toType: 1, toID: fixture.contactID, text: fixture.firstMessage, sentAt: "2026-08-20 14:20:00"},
		{key: "room", toType: 2, toID: fixture.roomID, roomID: fixture.roomID, text: fixture.secondMsg, sentAt: "2026-08-19 16:40:00"},
	}
	for offset, message := range messages {
		msgid := fmt.Sprintf("%s-%s-%s", fixturePrefix, fixture.key, message.key)
		if cleanup {
			if _, err := tx.ExecContext(ctx, `DELETE FROM mc_work_message_1 WHERE corp_id=? AND msgid=?`, corpID, msgid); err != nil {
				return err
			}
			continue
		}
		payload, err := json.Marshal(map[string]string{"msgtype": "text", "text": message.text, "fixture": fixturePrefix})
		if err != nil {
			return err
		}
		content := string(payload)
		var id int
		err = tx.QueryRowContext(ctx, `SELECT id FROM mc_work_message_1 WHERE corp_id=? AND msgid=? LIMIT 1`, corpID, msgid).Scan(&id)
		if err == sql.ErrNoRows {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO mc_work_message_1 (corp_id, msgid, seq, work_employee_id, to_user_type, to_user_id, sender_type, action, type, msg_type, content, content_text, room_id, status, msg_data_time, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, 0, 0, 1, 1, ?, ?, ?, 0, ?, NOW(), NOW())
			`, corpID, msgid, baseSeq+int64(offset), employeeID, message.toType, message.toID, content, message.text, message.roomID, message.sentAt)
		} else if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE mc_work_message_1 SET work_employee_id=?, to_user_type=?, to_user_id=?, room_id=?, content=?, content_text=?, msg_data_time=?, deleted_at=NULL, updated_at=NOW() WHERE id=? AND corp_id=?`, employeeID, message.toType, message.toID, message.roomID, content, message.text, message.sentAt, id, corpID)
		}
		if err != nil {
			return fmt.Errorf("upsert message %s: %w", msgid, err)
		}
	}
	return nil
}

func removeFixture(ctx context.Context, tx *sql.Tx, corpID int) error {
	for _, fixture := range fixtures {
		id, err := employeeID(ctx, tx, corpID, fixture.key)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM mc_work_message_1 WHERE corp_id=? AND msgid LIKE ?`, corpID, fixturePrefix+"-"+fixture.key+"-%"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM mc_work_employee_department WHERE employee_id=?`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM mc_work_employee WHERE id=? AND corp_id=?`, id, corpID); err != nil {
			return err
		}
	}
	return nil
}
