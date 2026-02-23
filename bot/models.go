package main

import "time"

type Message struct {
	ID         int
	AnonNumber int
	UserID     int64
	Username   string
	CreatedAt  time.Time
}

type UserLimit struct {
	UserID int64
	Count  int
	Date   string
}
