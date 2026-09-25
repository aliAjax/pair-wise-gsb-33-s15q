package model

import "time"

// AnswerLike records one user's support for an answer. The unique index on
// (user_id, answer_id) guarantees that a user can only like an answer once,
// even when two requests arrive at the same time.
type AnswerLike struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index:idx_answer_like_user_answer,unique;not null" json:"user_id"`
	AnswerID  uint      `gorm:"index:idx_answer_like_user_answer,unique;index;not null" json:"answer_id"`
	CreatedAt time.Time `json:"created_at"`
}
