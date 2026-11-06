//go:build !windows

package main

import "errors"

func protectAISecret([]byte) ([]byte, error) {
	return nil, errors.New("AI credential persistence requires Windows")
}
func unprotectAISecret([]byte) ([]byte, error) {
	return nil, errors.New("AI credential persistence requires Windows")
}
