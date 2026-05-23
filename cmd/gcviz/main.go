package main

import "os"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		if ee := (ExitError{}); AsExitError(err, &ee) {
			os.Exit(ee.Code)
		}
		os.Exit(1)
	}
}
