package repository

import (
	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

// AnswerLikeRepository handles persistence of answer like records.
type AnswerLikeRepository struct {
	db *gorm.DB
}

// NewAnswerLikeRepository creates an AnswerLikeRepository.
func NewAnswerLikeRepository(db *gorm.DB) *AnswerLikeRepository {
	return &AnswerLikeRepository{db: db}
}

// Exists reports whether a user currently likes an answer.
func (r *AnswerLikeRepository) Exists(userID, answerID uint) (bool, error) {
	var count int64
	if err := r.db.Model(&model.AnswerLike{}).
		Where("user_id = ? AND answer_id = ?", userID, answerID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ExistsTx reports whether a user currently likes an answer within an outer transaction.
func (r *AnswerLikeRepository) ExistsTx(tx *gorm.DB, userID, answerID uint) (bool, error) {
	var count int64
	if err := tx.Model(&model.AnswerLike{}).
		Where("user_id = ? AND answer_id = ?", userID, answerID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateTx inserts a like record within an outer transaction.
func (r *AnswerLikeRepository) CreateTx(tx *gorm.DB, userID, answerID uint) error {
	if err := tx.Create(&model.AnswerLike{UserID: userID, AnswerID: answerID}).Error; err != nil {
		if isDuplicate(err) {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

// DeleteTx removes a like record within an outer transaction and reports
// whether a row was actually deleted.
func (r *AnswerLikeRepository) DeleteTx(tx *gorm.DB, userID, answerID uint) (bool, error) {
	res := tx.Where("user_id = ? AND answer_id = ?", userID, answerID).
		Delete(&model.AnswerLike{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// LikedAnswerIDs returns the subset of answerIDs that the user has liked.
func (r *AnswerLikeRepository) LikedAnswerIDs(userID uint, answerIDs []uint) (map[uint]bool, error) {
	result := make(map[uint]bool)
	if userID == 0 || len(answerIDs) == 0 {
		return result, nil
	}
	var ids []uint
	if err := r.db.Model(&model.AnswerLike{}).
		Where("user_id = ? AND answer_id IN ?", userID, answerIDs).
		Pluck("answer_id", &ids).Error; err != nil {
		return nil, err
	}
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}
