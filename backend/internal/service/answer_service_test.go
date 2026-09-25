package service

import (
	"errors"
	"log/slog"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/gbplantwiki/gbplantwiki/internal/repository"
)

func newLikeMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	return db, mock
}

func answerRows(count int) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "question_id", "user_id", "content", "is_best", "like_count"}).
		AddRow(7, 3, 1, "ok", false, count)
}

func TestAnswerToggleLikeAddsOnce(t *testing.T) {
	db, mock := newLikeMockDB(t)
	svc := NewAnswerService(db, repository.NewAnswerRepository(db),
		repository.NewAnswerLikeRepository(db), repository.NewQuestionRepository(db), slog.Default())

	// FindByID before toggle
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `answers` WHERE `answers`.`id` = ? ORDER BY `answers`.`id` LIMIT ?")).
		WithArgs(uint64(7), 1).WillReturnRows(answerRows(0))
	mock.ExpectBegin()
	// no existing record
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `answer_likes`").
		WithArgs(uint64(5), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(0))
	mock.ExpectExec("INSERT INTO `answer_likes`").
		WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectExec("UPDATE `answers` SET `like_count`=like_count \\+ \\? WHERE id = \\?").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	// reload
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `answers` WHERE `answers`.`id` = ? ORDER BY `answers`.`id` LIMIT ?")).
		WithArgs(uint64(7), 1).WillReturnRows(answerRows(1))

	a, liked, err := svc.ToggleLike(5, 7)
	if err != nil {
		t.Fatalf("ToggleLike: %v", err)
	}
	if !liked || a.LikeCount != 1 {
		t.Errorf("expected liked=true count=1, got liked=%v count=%d", liked, a.LikeCount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAnswerToggleLikeConcurrentDoubleLikeIsIdempotent(t *testing.T) {
	db, mock := newLikeMockDB(t)
	svc := NewAnswerService(db, repository.NewAnswerRepository(db),
		repository.NewAnswerLikeRepository(db), repository.NewQuestionRepository(db), slog.Default())

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `answers` WHERE `answers`.`id` = ? ORDER BY `answers`.`id` LIMIT ?")).
		WithArgs(uint64(7), 1).WillReturnRows(answerRows(0))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `answer_likes`").
		WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(0))
	// The racing request already inserted the (user, answer) row: the unique
	// index rejects our insert. The service must not bump the count again.
	mock.ExpectExec("INSERT INTO `answer_likes`").
		WillReturnError(errors.New("Error 1062 (23000): Duplicate entry '5-7' for key 'uk_answer_like_user_answer'"))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `answers` WHERE `answers`.`id` = ? ORDER BY `answers`.`id` LIMIT ?")).
		WithArgs(uint64(7), 1).WillReturnRows(answerRows(1))

	a, liked, err := svc.ToggleLike(5, 7)
	if err != nil {
		t.Fatalf("ToggleLike duplicate: %v", err)
	}
	if !liked || a.LikeCount != 1 {
		t.Errorf("concurrent double like must keep one support: liked=%v count=%d", liked, a.LikeCount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAnswerToggleUnlikeDecrements(t *testing.T) {
	db, mock := newLikeMockDB(t)
	svc := NewAnswerService(db, repository.NewAnswerRepository(db),
		repository.NewAnswerLikeRepository(db), repository.NewQuestionRepository(db), slog.Default())

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `answers` WHERE `answers`.`id` = ? ORDER BY `answers`.`id` LIMIT ?")).
		WithArgs(uint64(7), 1).WillReturnRows(answerRows(1))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `answer_likes`").
		WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(1))
	mock.ExpectExec("DELETE FROM `answer_likes`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `answers` SET `like_count`=GREATEST\\(like_count \\+ \\?, 0\\) WHERE id = \\?").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `answers` WHERE `answers`.`id` = ? ORDER BY `answers`.`id` LIMIT ?")).
		WithArgs(uint64(7), 1).WillReturnRows(answerRows(0))

	a, liked, err := svc.ToggleLike(5, 7)
	if err != nil {
		t.Fatalf("ToggleLike unlike: %v", err)
	}
	if liked || a.LikeCount != 0 {
		t.Errorf("expected liked=false count=0, got liked=%v count=%d", liked, a.LikeCount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestAnswerToggleLikeMissingAnswer(t *testing.T) {
	db, mock := newLikeMockDB(t)
	svc := NewAnswerService(db, repository.NewAnswerRepository(db),
		repository.NewAnswerLikeRepository(db), repository.NewQuestionRepository(db), slog.Default())

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `answers` WHERE `answers`.`id` = ? ORDER BY `answers`.`id` LIMIT ?")).
		WithArgs(uint64(99), 1).WillReturnError(gorm.ErrRecordNotFound)

	if _, _, err := svc.ToggleLike(5, 99); err == nil {
		t.Error("expected error for missing answer")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
