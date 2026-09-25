package repository

import (
	"errors"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

// AnswerRepository handles persistence of answers.
type AnswerRepository struct {
	db *gorm.DB
}

// NewAnswerRepository creates an AnswerRepository.
func NewAnswerRepository(db *gorm.DB) *AnswerRepository {
	return &AnswerRepository{db: db}
}

// Create inserts an answer.
func (r *AnswerRepository) Create(a *model.Answer) error {
	return r.db.Create(a).Error
}

// FindByID locates an answer by id.
func (r *AnswerRepository) FindByID(id uint) (*model.Answer, error) {
	var a model.Answer
	if err := r.db.First(&a, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// ListByQuestion returns answers for a question.
func (r *AnswerRepository) ListByQuestion(questionID uint) ([]model.Answer, error) {
	var items []model.Answer
	if err := r.db.Where("question_id = ?", questionID).Order("is_best DESC, like_count DESC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Update persists an answer.
func (r *AnswerRepository) Update(a *model.Answer) error {
	return r.db.Save(a).Error
}

// UpdateTx persists an answer within an outer transaction.
func (r *AnswerRepository) UpdateTx(tx *gorm.DB, a *model.Answer) error {
	return tx.Save(a).Error
}

// CreateLikeTx inserts a like record within an outer transaction.
func (r *AnswerRepository) CreateLikeTx(tx *gorm.DB, like *model.AnswerLike) error {
	if err := tx.Create(like).Error; err != nil {
		if isDuplicate(err) {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

// DeleteLikeTx removes a like record within an outer transaction, reporting whether one existed.
func (r *AnswerRepository) DeleteLikeTx(tx *gorm.DB, userID, answerID uint) (bool, error) {
	res := tx.Where("user_id = ? AND answer_id = ?", userID, answerID).Delete(&model.AnswerLike{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// AddLikeCountTx shifts the like count of an answer within an outer transaction.
func (r *AnswerRepository) AddLikeCountTx(tx *gorm.DB, id uint, delta int) error {
	q := tx.Model(&model.Answer{}).Where("id = ?", id)
	if delta < 0 {
		q = q.Where("like_count > 0")
	}
	return q.UpdateColumn("like_count", gorm.Expr("like_count + ?", delta)).Error
}

// ListLikedAnswerIDs returns the ids among answerIDs that the user has liked.
func (r *AnswerRepository) ListLikedAnswerIDs(userID uint, answerIDs []uint) ([]uint, error) {
	ids := []uint{}
	if len(answerIDs) == 0 {
		return ids, nil
	}
	err := r.db.Model(&model.AnswerLike{}).
		Where("user_id = ? AND answer_id IN ?", userID, answerIDs).
		Pluck("answer_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// ClearBestForQuestion resets best answers for a question.
func (r *AnswerRepository) ClearBestForQuestion(questionID uint) error {
	return r.db.Model(&model.Answer{}).Where("question_id = ?", questionID).
		Update("is_best", false).Error
}

// ClearBestForQuestionTx resets best answers within an outer transaction.
func (r *AnswerRepository) ClearBestForQuestionTx(tx *gorm.DB, questionID uint) error {
	return tx.Model(&model.Answer{}).Where("question_id = ?", questionID).
		Update("is_best", false).Error
}
