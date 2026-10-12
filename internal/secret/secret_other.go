//go:build !darwin && !windows

package secret

import "errors"

var errUnsupported = errors.New("no credential store on this platform")

func get(string) (string, error) { return "", ErrNotFound }
func set(string, string) error   { return errUnsupported }
func del(string) error           { return ErrNotFound }
