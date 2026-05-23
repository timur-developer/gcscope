package main

import "errors"

type ExitError struct {
	Code int
	Err  error
}

func (e ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "exit"
}

func (e ExitError) Unwrap() error {
	return e.Err
}

func AsExitError(err error, target *ExitError) bool {
	return errors.As(err, target)
}
