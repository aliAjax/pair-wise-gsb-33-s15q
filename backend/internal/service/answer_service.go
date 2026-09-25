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

// AnswerService implements answer creation, like toggle and adoption.
type AnswerService struct {
	db           *gorm.DB
	repo         *repository.AnswerRepository
	likeRepo     *repository.AnswerLikeRepository
	questionRepo *repository.QuestionRepository
	logger       *slog.Logger
}

// NewAnswerService creates an AnswerService.
func NewAnswerService(db *gorm.DB, repo *repository.AnswerRepository, likeRepo *repository.AnswerLikeRepository, questionRepo *repository.QuestionRepository, logger *slog.Logger) *AnswerService {
	return &AnswerService{db: db, repo: repo, likeRepo: likeRepo, questionRepo: questionRepo, logger: logger}
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

// ListByQuestion returns answers for a question together with the set of
// answer ids the given user has liked. A zero userID (anonymous) yields an
// empty set.
func (s *AnswerService) ListByQuestion(userID, questionID uint) ([]model.Answer, map[uint]bool, error) {
	items, err := s.repo.ListByQuestion(questionID)
	if err != nil {
		return nil, nil, fmt.Errorf("answer list: %w", err)
	}
	ids := make([]uint, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
	}
	liked, err := s.likeRepo.LikedAnswerIDs(userID, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("answer list liked: %w", err)
	}
	return items, liked, nil
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

// ToggleLike adds the user's support to an answer or removes it when already
// present. The like record and the denormalized like_count are updated in one
// transaction. The unique (user_id, answer_id) constraint makes concurrent
// double likes idempotent: at most one record survives and the count is
// incremented at most once. It returns the updated answer and the resulting
// liked state.
func (s *AnswerService) ToggleLike(userID, answerID uint) (*model.Answer, bool, error) {
	if _, err := s.repo.FindByID(answerID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, false, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Answer[id=%d] not found", answerID))
		}
		return nil, false, fmt.Errorf("answer like find: %w", err)
	}

	var liked bool
	err := s.db.Transaction(func(tx *gorm.DB) error {
		exists, err := s.likeRepo.ExistsTx(tx, userID, answerID)
		if err != nil {
			return fmt.Errorf("answer like check: %w", err)
		}
		if !exists {
			if err := s.likeRepo.CreateTx(tx, userID, answerID); err != nil {
				if errors.Is(err, repository.ErrDuplicate) {
					// A concurrent request already recorded the like; keep
					// that single support and leave the count untouched.
					liked = true
					return nil
				}
				return fmt.Errorf("answer like create: %w", err)
			}
			if err := s.repo.AdjustLikeCountTx(tx, answerID, 1); err != nil {
				return fmt.Errorf("answer like count: %w", err)
			}
			liked = true
			return nil
		}

		deleted, err := s.likeRepo.DeleteTx(tx, userID, answerID)
		if err != nil {
			return fmt.Errorf("answer unlike delete: %w", err)
		}
		if deleted {
			if err := s.repo.AdjustLikeCountTx(tx, answerID, -1); err != nil {
				return fmt.Errorf("answer unlike count: %w", err)
			}
		}
		liked = false
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	a, err := s.repo.FindByID(answerID)
	if err != nil {
		return nil, false, fmt.Errorf("answer like reload: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogAnswerLikeSuccess, answerID), "user_id", userID, "liked", liked)
	return a, liked, nil
}
