package task

import "errors"

var (
	ErrNotFound         = errors.New("task not found")
	ErrTemplateNotFound = errors.New("task template not found")
)
