package models

import "time"

// Comment は動画に付けられたコメントを表す
type Comment struct {
	ID        string    `json:"id"`
	VideoID   string    `json:"video_id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
