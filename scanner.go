package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
)

// scanner is a wrapper around [bufio.Scanner] that allows reading input with context cancellation.
type scanner struct {
	*bufio.Scanner
}

func newScanner(r io.Reader) *scanner {
	return &scanner{
		Scanner: bufio.NewScanner(r),
	}
}

// Read reads input from the scanner, respecting the provided context.
// It returns an error if the context is canceled or if there is an error scanning the input.
func (s *scanner) Read(ctx context.Context) error {
	scanned := make(chan struct{})
	errChan := make(chan error)
	defer func() {
		// clean up channels when done
		close(errChan)
		close(scanned)
	}()

	// because bufio.Scanner does not support context cancellation directly,
	// we run the scanning in a goroutine and use channels to communicate completion or errors.
	go func() {
		s.Scan()
		err := s.Err()
		if err != nil {
			errChan <- fmt.Errorf("error scanning input: %v", err)
			return
		}
		// this will signal that scanning is done
		scanned <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	case <-scanned:
		// Successfully scanned input
		return nil
	}
}
