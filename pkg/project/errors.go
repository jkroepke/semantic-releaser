package project

import (
	"errors"
)

var (
	ErrProjectFileNotFound = errors.New("file Project.yaml not found")
	ErrMultipleMatchInTag  = errors.New("multiple matches in tag")
)
