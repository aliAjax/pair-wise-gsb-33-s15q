package service

import (
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

// AnswerService implements answer creation, like and adoption.
type AnswerService struct {
	db           *gorm.DB
	repo         *repository.AnswerRepository
	questionRepo *repository.QuestionRepository
	logger       *slog.Logger
}

// NewAnswerService creates an AnswerService.
func NewAnswerService(db *gorm.DB, repo *repository.AnswerRepository, questionRepo *repository.QuestionRepository, logger *slog.Logger) *AnswerService {
	return &AnswerService{db: db, repo: repo, questionRepo: questionRepo, logger: logger}
}

// Create adds an answer to a question.
func (s *AnswerService) Create(userID, questionID uint, content string) (*model.Answer, error) {
	if _, err := s.questionRepo.FindByID(questionID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Question[id=%d] not found", questionID))
		}
		return nil, fmt.Errorf("answer create question find: %w", err)
	}
	a := &model.Answer{QuestionID: questionID, UserID: userID, Content: content}
	if err := s.repo.Create(a); err != nil {
		return nil, fmt.Errorf("answer create: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogAnswerCreateSuccess, questionID), "id", a.ID)
	return a, nil
}

// ListByQuestion returns answers for a question, marking which ones the user liked.
func (s *AnswerService) ListByQuestion(questionID, userID uint) ([]model.Answer, error) {
	items, err := s.repo.ListByQuestion(questionID)
	if err != nil {
		return nil, fmt.Errorf("answer list: %w", err)
	}
	if userID == 0 || len(items) == 0 {
		return items, nil
	}
	ids := make([]uint, len(items))
	for i, a := range items {
		ids[i] = a.ID
	}
	likedIDs, err := s.repo.ListLikedAnswerIDs(userID, ids)
	if err != nil {
		return nil, fmt.Errorf("answer list liked: %w", err)
	}
	liked := make(map[uint]bool, len(likedIDs))
	for _, id := range likedIDs {
		liked[id] = true
	}
	for i := range items {
		items[i].LikedByMe = liked[items[i].ID]
	}
	return items, nil
}

// Adopt marks an answer as the best answer. Only the question owner may adopt.
func (s *AnswerService) Adopt(userID, questionID, answerID uint) (*model.Answer, error) {
	q, err := s.questionRepo.FindByID(questionID)
	if err != nil {
		return nil, fmt.Errorf("answer adopt question find: %w", err)
	}
	if q.UserID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("Answer[question_id=%d] adopt failed: user_id=%d not question owner", questionID, userID))
	}
	a, err := s.repo.FindByID(answerID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Answer[id=%d] not found", answerID))
		}
		return nil, fmt.Errorf("answer adopt find: %w", err)
	}
	if a.QuestionID != questionID {
		return nil, util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("Answer[id=%d] adopt failed: question_id mismatch", answerID))
	}
	a.IsBest = true
	q.Status = "closed"
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.ClearBestForQuestionTx(tx, questionID); err != nil {
			return fmt.Errorf("answer adopt clear best: %w", err)
		}
		if err := s.repo.UpdateTx(tx, a); err != nil {
			return fmt.Errorf("answer adopt update: %w", err)
		}
		if err := s.questionRepo.UpdateTx(tx, q); err != nil {
			return fmt.Errorf("answer adopt question update: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(fmt.Sprintf(constants.LogAnswerAdoptSuccess, answerID), "question_id", questionID)
	return a, nil
}

// ToggleLike adds or removes the user's like on an answer. A repeated click
// withdraws the support, and the unique like record keeps a concurrent double
// click from the same user counting more than once.
func (s *AnswerService) ToggleLike(userID, answerID uint) (*model.Answer, error) {
	if _, err := s.repo.FindByID(answerID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Answer[id=%d] not found", answerID))
		}
		return nil, fmt.Errorf("answer like find: %w", err)
	}
	liked := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		removed, err := s.repo.DeleteLikeTx(tx, userID, answerID)
		if err != nil {
			return fmt.Errorf("answer like remove: %w", err)
		}
		if removed {
			return s.repo.AddLikeCountTx(tx, answerID, -1)
		}
		like := &model.AnswerLike{AnswerID: answerID, UserID: userID}
		if err := s.repo.CreateLikeTx(tx, like); err != nil {
			if errors.Is(err, repository.ErrDuplicate) {
				// A concurrent click from the same user already recorded the
				// support; keep exactly one like and leave the count untouched.
				liked = true
				return nil
			}
			return fmt.Errorf("answer like create: %w", err)
		}
		liked = true
		return s.repo.AddLikeCountTx(tx, answerID, 1)
	})
	if err != nil {
		return nil, err
	}
	a, err := s.repo.FindByID(answerID)
	if err != nil {
		return nil, fmt.Errorf("answer like reload: %w", err)
	}
	a.LikedByMe = liked
	if liked {
		s.logger.Info(fmt.Sprintf(constants.LogAnswerLikeSuccess, answerID), "user_id", userID)
	} else {
		s.logger.Info(fmt.Sprintf(constants.LogAnswerUnlikeSuccess, answerID), "user_id", userID)
	}
	return a, nil
}
