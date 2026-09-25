package dto

import "time"

// QuestionCreateRequest creates a Q&A question.
type QuestionCreateRequest struct {
	Title   string `json:"title" binding:"required,max=255"`
	Content string `json:"content" binding:"required"`
	Images  string `json:"images"`
}

// AnswerCreateRequest creates an answer.
type AnswerCreateRequest struct {
	Content string `json:"content" binding:"required"`
}

// AdoptRequest marks an answer as best.
type AdoptRequest struct {
	AnswerID uint `json:"answer_id" binding:"required"`
}

// AnswerResponse is an answer with the current viewer's like state.
type AnswerResponse struct {
	ID         uint      `json:"id"`
	QuestionID uint      `json:"question_id"`
	UserID     uint      `json:"user_id"`
	Content    string    `json:"content"`
	IsBest     bool      `json:"is_best"`
	LikeCount  int       `json:"like_count"`
	Liked      bool      `json:"liked"`
	CreatedAt  time.Time `json:"created_at"`
}

// LikeResponse is returned by the like toggle endpoint.
type LikeResponse struct {
	LikeCount int  `json:"like_count"`
	Liked     bool `json:"liked"`
}
