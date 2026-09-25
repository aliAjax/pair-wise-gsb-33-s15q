package model

import "time"

// AnswerLike records one user's support for an answer. The unique index on
// (answer_id, user_id) guarantees a user keeps at most one like per answer.
type AnswerLike struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AnswerID  uint      `gorm:"index:uk_answer_like_user,unique;not null" json:"answer_id"`
	UserID    uint      `gorm:"index:uk_answer_like_user,unique;not null" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}
