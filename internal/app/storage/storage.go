package storage

import "errors"

var (
	ErrLoginExists   = errors.New("login exist")
	ErrLoginNotFound = errors.New("login not found")
	ErrOrderExists   = errors.New("order exist")
)
