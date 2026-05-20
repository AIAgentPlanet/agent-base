package dao

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"github.com/go-dev-frame/sponge/pkg/sgorm/query"
	"github.com/go-dev-frame/sponge/pkg/gotest"
	"github.com/go-dev-frame/sponge/pkg/utils"

	"agent-base/services/user-service/internal/cache"
	"agent-base/services/user-service/internal/database"
	"agent-base/services/user-service/internal/model"
)

func newUsersDao() *gotest.Dao {
	testData := &model.Users{}
	testData.ID = 1
	// you can set the other fields of testData here, such as:
	//testData.CreatedAt = time.Now()
	//testData.UpdatedAt = testData.CreatedAt

	// init mock cache
	//c := gotest.NewCache(map[string]interface{}{"no cache": testData}) // to test mysql, disable caching
	c := gotest.NewCache(map[string]interface{}{utils.Uint64ToStr(testData.ID): testData})
	c.ICache = cache.NewUsersCache(&database.CacheType{
		CType: "redis",
		Rdb:   c.RedisClient,
	})

	// init mock dao
	d := gotest.NewDao(c, testData)
	d.IDao = NewUsersDao(d.DB, c.ICache.(cache.UsersCache))

	return d
}

func Test_usersDao_Create(t *testing.T) {
	d := newUsersDao()
	defer d.Close()
	testData := d.TestData.(*model.Users)

	d.SQLMock.ExpectBegin()
	d.SQLMock.ExpectExec("INSERT INTO .*").
		WithArgs(d.GetAnyArgs(testData)...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	d.SQLMock.ExpectCommit()

	err := d.IDao.(UsersDao).Create(d.Ctx, testData)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_usersDao_DeleteByID(t *testing.T) {
	d := newUsersDao()
	defer d.Close()
	testData := d.TestData.(*model.Users)
	expectedSQLForDeletion := "DELETE .*"

	d.SQLMock.ExpectBegin()
	d.SQLMock.ExpectExec(expectedSQLForDeletion).
		WithArgs(d.AnyTime, testData.ID).
		WillReturnResult(sqlmock.NewResult(int64(testData.ID), 1))
	d.SQLMock.ExpectCommit()

	err := d.IDao.(UsersDao).DeleteByID(d.Ctx, testData.ID)
	if err != nil {
		t.Fatal(err)
	}

	// zero id error
	err = d.IDao.(UsersDao).DeleteByID(d.Ctx, 0)
	assert.Error(t, err)
}

func Test_usersDao_UpdateByID(t *testing.T) {
	d := newUsersDao()
	defer d.Close()
	testData := d.TestData.(*model.Users)

	d.SQLMock.ExpectBegin()
	d.SQLMock.ExpectExec("UPDATE .*").
		WithArgs(d.AnyTime, testData.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	d.SQLMock.ExpectCommit()

	err := d.IDao.(UsersDao).UpdateByID(d.Ctx, testData)
	if err != nil {
		t.Fatal(err)
	}

	// zero id error
	err = d.IDao.(UsersDao).UpdateByID(d.Ctx, &model.Users{})
	assert.Error(t, err)
	
}

func Test_usersDao_GetByID(t *testing.T) {
	d := newUsersDao()
	defer d.Close()
	testData := d.TestData.(*model.Users)

	// column names and corresponding data
	rows := sqlmock.NewRows([]string{"id"}).
		AddRow(testData.ID)

	d.SQLMock.ExpectQuery("SELECT .*").
		WithArgs(testData.ID, 1).
		WillReturnRows(rows)

	_, err := d.IDao.(UsersDao).GetByID(d.Ctx, testData.ID)
	if err != nil {
		t.Fatal(err)
	}

	err = d.SQLMock.ExpectationsWereMet()
	if err != nil {
		t.Fatal(err)
	}

	// notfound error
	d.SQLMock.ExpectQuery("SELECT .*").
		WithArgs(2).
		WillReturnRows(rows)
	_, err = d.IDao.(UsersDao).GetByID(d.Ctx, 2)
	assert.Error(t, err)

	d.SQLMock.ExpectQuery("SELECT .*").
		WithArgs(3, 4).
		WillReturnRows(rows)
	_, err = d.IDao.(UsersDao).GetByID(d.Ctx, 4)
	assert.Error(t, err)
}

func Test_usersDao_GetByColumns(t *testing.T) {
	d := newUsersDao()
	defer d.Close()
	testData := d.TestData.(*model.Users)

	rows := sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
		AddRow(testData.ID, testData.CreatedAt, testData.UpdatedAt)

	d.SQLMock.ExpectQuery("SELECT .*").WillReturnRows(rows)

	_, _, err := d.IDao.(UsersDao).GetByColumns(d.Ctx, &query.Params{
		Page: 0,
		Limit: 10,
		Sort: "ignore count", // ignore test count(*)
	})
	if err != nil {
		t.Fatal(err)
	}

	err = d.SQLMock.ExpectationsWereMet()
	if err != nil {
		t.Fatal(err)
	}

	// err test
	_, _, err = d.IDao.(UsersDao).GetByColumns(d.Ctx, &query.Params{
		Page: 0,
		Limit: 10,
		Columns: []query.Column{
			{
				Name:  "id",
				Exp:   "<",
				Value: 0,
			},
		},
	})
	assert.Error(t, err)

	// error test
	dao := &usersDao{}
	_, _, err = dao.GetByColumns(context.Background(), &query.Params{Columns: []query.Column{{}}})
	t.Log(err)
}

func Test_usersDao_DeleteByIDs(t *testing.T) {
	d := newUsersDao()
	defer d.Close()
	testData := d.TestData.(*model.Users)

	d.SQLMock.ExpectBegin()
	d.SQLMock.ExpectExec("UPDATE .*").
		WithArgs(d.AnyTime, testData.ID).
		WillReturnResult(sqlmock.NewResult(int64(testData.ID), 1))
	d.SQLMock.ExpectCommit()

	err := d.IDao.(UsersDao).DeleteByID(d.Ctx, testData.ID)
	if err != nil {
		t.Fatal(err)
	}

	// zero id error
	err = d.IDao.(UsersDao).DeleteByIDs(d.Ctx, []uint64{0})
	assert.Error(t, err)
}

func Test_usersDao_GetByCondition(t *testing.T) {
	d := newUsersDao()
	defer d.Close()
	testData := d.TestData.(*model.Users)

	// column names and corresponding data
	rows := sqlmock.NewRows([]string{"id"}).
		AddRow(testData.ID)

	d.SQLMock.ExpectQuery("SELECT .*").
		WithArgs(testData.ID).
		WillReturnRows(rows)

	_, err := d.IDao.(UsersDao).GetByCondition(d.Ctx, &query.Conditions{
		Columns: []query.Column{
			{
				Name:  "id",
				Value: testData.ID,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = d.SQLMock.ExpectationsWereMet()
	if err != nil {
		t.Fatal(err)
	}

	// notfound error
	d.SQLMock.ExpectQuery("SELECT .*").
		WithArgs(2).
		WillReturnRows(rows)
	_, err = d.IDao.(UsersDao).GetByCondition(d.Ctx, &query.Conditions{
		Columns: []query.Column{
			{
				Name:  "id",
				Value: 2,
			},
		},
	})
	assert.Error(t, err)
}

func Test_usersDao_GetByIDs(t *testing.T) {
	d := newUsersDao()
	defer d.Close()
	testData := d.TestData.(*model.Users)

	rows := sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
		AddRow(testData.ID, testData.CreatedAt, testData.UpdatedAt)

	d.SQLMock.ExpectQuery("SELECT .*").
		WithArgs(testData.ID).
		WillReturnRows(rows)

	_, err := d.IDao.(UsersDao).GetByIDs(d.Ctx, []uint64{testData.ID})
	if err != nil {
		t.Fatal(err)
	}

	_, err = d.IDao.(UsersDao).GetByIDs(d.Ctx, []uint64{111})
	assert.Error(t, err)

	err = d.SQLMock.ExpectationsWereMet()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_usersDao_GetByLastID(t *testing.T) {
	d := newUsersDao()
	defer d.Close()
	testData := d.TestData.(*model.Users)

	rows := sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
		AddRow(testData.ID, testData.CreatedAt, testData.UpdatedAt)

	d.SQLMock.ExpectQuery("SELECT .*").WillReturnRows(rows)

	_, err := d.IDao.(UsersDao).GetByLastID(d.Ctx, 0, 10, "")
	if err != nil {
		t.Fatal(err)
	}

	err = d.SQLMock.ExpectationsWereMet()
	if err != nil {
		t.Fatal(err)
	}

	// err test
	_, err = d.IDao.(UsersDao).GetByLastID(d.Ctx, 0, 10, "unknown-column")
	assert.Error(t, err)
}

func Test_usersDao_CreateByTx(t *testing.T) {
	d := newUsersDao()
	defer d.Close()
	testData := d.TestData.(*model.Users)

	d.SQLMock.ExpectBegin()
	d.SQLMock.ExpectExec("INSERT INTO .*").
		WithArgs(d.GetAnyArgs(testData)...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	d.SQLMock.ExpectCommit()

	_, err := d.IDao.(UsersDao).CreateByTx(d.Ctx, d.DB, testData)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_usersDao_DeleteByTx(t *testing.T) {
	d := newUsersDao()
	defer d.Close()
	testData := d.TestData.(*model.Users)
	expectedSQLForDeletion := "DELETE .*"

	d.SQLMock.ExpectBegin()
	d.SQLMock.ExpectExec(expectedSQLForDeletion).
		WithArgs(d.AnyTime, testData.ID).
		WillReturnResult(sqlmock.NewResult(int64(testData.ID), 1))
	d.SQLMock.ExpectCommit()

	err := d.IDao.(UsersDao).DeleteByTx(d.Ctx, d.DB, testData.ID)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_usersDao_UpdateByTx(t *testing.T) {
	d := newUsersDao()
	defer d.Close()
	testData := d.TestData.(*model.Users)

	d.SQLMock.ExpectBegin()
	d.SQLMock.ExpectExec("UPDATE .*").
		WithArgs(d.AnyTime, testData.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	d.SQLMock.ExpectCommit()

	err := d.IDao.(UsersDao).UpdateByTx(d.Ctx, d.DB, testData)
	if err != nil {
		t.Fatal(err)
	}
}
