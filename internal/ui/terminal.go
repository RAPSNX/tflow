package ui

import "golang.org/x/term"

type termState = term.State

func enterRaw(fd int) (*termState, error) {
	return term.MakeRaw(fd)
}

func leaveRaw(fd int, state *termState) error {
	return term.Restore(fd, state)
}
