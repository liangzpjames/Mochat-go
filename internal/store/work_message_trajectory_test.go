package store

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestTrajectoryTargetsDoesNotDependOnRoomAvatarColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, COALESCE(name,''), '' FROM mc_work_room WHERE corp_id=? AND id IN (?) AND deleted_at IS NULL`)).
		WithArgs(1, 3001).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "avatar"}).AddRow(3001, "华东销售线索群", ""))

	targets, err := NewMySQLStore(db).trajectoryTargets(context.Background(), 1, []trajectoryEventRow{{Type: 2, ID: 3001}})
	if err != nil {
		t.Fatal(err)
	}
	if got := targets[trajectoryTargetKey(2, 3001)]; got.Status != "available" || got.Name != "华东销售线索群" {
		t.Fatalf("room target=%+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
