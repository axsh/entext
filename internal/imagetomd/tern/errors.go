package tern

import "errors"

var (
	ErrBootFailed    = errors.New("tern in-process boot failed")
	ErrConnectFailed = errors.New("tern external connect failed")
	ErrConfigMissing = errors.New("tern config file not found")
)
