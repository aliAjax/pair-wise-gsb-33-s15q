package service

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/gbplantwiki/gbplantwiki/internal/repository"
)

func expectAnswerRow(mock sqlmock.Sqlmock, id, likeCount uint) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `answers` WHERE `answers`.`id` = ? ORDER BY `answers`.`id` LIMIT ?")).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "question_id", "user_id", "content", "is_best", "like_count"}).
			AddRow(id, 1, 2, "content", 0, likeCount))
}

func TestAnswerServiceToggleLikeAdds(t *testing.T) {
	db, mock := newServiceDB(t)
	answerRepo := repository.NewAnswerRepository(db)
	questionRepo := repository.NewQuestionRepository(db)
	svc := NewAnswerService(db, answerRepo, questionRepo, newTestLogger())

	const userID, answerID = uint(7), uint(10)
	expectAnswerRow(mock, answerID, 5)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `answer_likes` WHERE user_id = ? AND answer_id = ?")).
		WithArgs(userID, answerID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `answer_likes` (`answer_id`,`user_id`,`created_at`) VALUES (?,?,?)")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `answers` SET `like_count`=like_count + ? WHERE id = ?")).
		WithArgs(1, answerID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectAnswerRow(mock, answerID, 6)

	a, err := svc.ToggleLike(userID, answerID)
	if err != nil {
		t.Fatalf("ToggleLike add: %v", err)
	}
	if !a.LikedByMe || a.LikeCount != 6 {
		t.Errorf("expected liked count 6, got liked=%v count=%d", a.LikedByMe, a.LikeCount)
	}
}

func TestAnswerServiceToggleLikeRemoves(t *testing.T) {
	db, mock := newServiceDB(t)
	answerRepo := repository.NewAnswerRepository(db)
	questionRepo := repository.NewQuestionRepository(db)
	svc := NewAnswerService(db, answerRepo, questionRepo, newTestLogger())

	const userID, answerID = uint(7), uint(10)
	expectAnswerRow(mock, answerID, 6)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `answer_likes` WHERE user_id = ? AND answer_id = ?")).
		WithArgs(userID, answerID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `answers` SET `like_count`=like_count + ? WHERE id = ? AND like_count > 0")).
		WithArgs(-1, answerID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectAnswerRow(mock, answerID, 5)

	a, err := svc.ToggleLike(userID, answerID)
	if err != nil {
		t.Fatalf("ToggleLike remove: %v", err)
	}
	if a.LikedByMe || a.LikeCount != 5 {
		t.Errorf("expected unliked count 5, got liked=%v count=%d", a.LikedByMe, a.LikeCount)
	}
}

func TestAnswerServiceToggleLikeConcurrentDuplicate(t *testing.T) {
	db, mock := newServiceDB(t)
	answerRepo := repository.NewAnswerRepository(db)
	questionRepo := repository.NewQuestionRepository(db)
	svc := NewAnswerService(db, answerRepo, questionRepo, newTestLogger())

	const userID, answerID = uint(7), uint(10)
	expectAnswerRow(mock, answerID, 5)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `answer_likes` WHERE user_id = ? AND answer_id = ?")).
		WithArgs(userID, answerID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `answer_likes` (`answer_id`,`user_id`,`created_at`) VALUES (?,?,?)")).
		WillReturnError(errors.New("Duplicate entry '10-7' for key 'uk_answer_like_user'"))
	mock.ExpectCommit()
	expectAnswerRow(mock, answerID, 5)

	a, err := svc.ToggleLike(userID, answerID)
	if err != nil {
		t.Fatalf("ToggleLike duplicate: %v", err)
	}
	if !a.LikedByMe || a.LikeCount != 5 {
		t.Errorf("concurrent double click must keep one like and count 5, got liked=%v count=%d", a.LikedByMe, a.LikeCount)
	}
}

func TestAnswerServiceListMarksLikedByMe(t *testing.T) {
	db, mock := newServiceDB(t)
	answerRepo := repository.NewAnswerRepository(db)
	questionRepo := repository.NewQuestionRepository(db)
	svc := NewAnswerService(db, answerRepo, questionRepo, newTestLogger())

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `answers` WHERE question_id = ? ORDER BY is_best DESC, like_count DESC, id ASC")).
		WithArgs(uint(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "question_id", "user_id", "content", "is_best", "like_count"}).
			AddRow(10, 1, 2, "a", 0, 5).
			AddRow(11, 1, 3, "b", 0, 8))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `answer_id` FROM `answer_likes` WHERE user_id = ? AND answer_id IN (?,?)")).
		WithArgs(uint(7), uint(10), uint(11)).
		WillReturnRows(sqlmock.NewRows([]string{"answer_id"}).AddRow(11))

	items, err := svc.ListByQuestion(1, 7)
	if err != nil {
		t.Fatalf("ListByQuestion: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 answers, got %d", len(items))
	}
	if items[0].LikedByMe || !items[1].LikedByMe {
		t.Errorf("liked flags wrong: %v %v", items[0].LikedByMe, items[1].LikedByMe)
	}
}
