package errors

import (
	stderrors "errors"
	"fmt"
	"strings"
	"testing"
)

func f1() error {
	return Wrap(NewInnerError(500, -1, "internal server error"), "inner")
}

func f2() error {
	return Wrap(f1(), "middle")
}

func f3() error {
	return Wrap(f2(), "outer")
}

func fn2() error {
	e1 := NewInnerError(500, -1, "internal server error")
	e2 := Wrap(e1, "inner")
	e3 := Wrap(e2, "middle")
	return WithMessage(e3, "outer")
}

func TestFn2(t *testing.T) {
	err := fn2()
	fmt.Printf("%+v\n", err) // with stack trace
}

func TestF3(t *testing.T) {
	err := f3()
	fmt.Printf("%+v\n", err) // with stack trace
}

func multiWrapStdError() error {
	base := stderrors.New("base")
	e1 := Wrap(base, "l1")
	e2 := Wrap(e1, "l2")
	return Wrap(e2, "l3")
}

func TestWrapPrintsInnermostStackOnly(t *testing.T) {
	formatted := fmt.Sprintf("%+v", multiWrapStdError())
	if got := strings.Count(formatted, "multiWrapStdError"); got != 1 {
		t.Fatalf("stack printed %d times, want 1\n%s", got, formatted)
	}
}

func TestWrapPrintsMessagesBeforeStack(t *testing.T) {
	formatted := fmt.Sprintf("%+v", multiWrapStdError())

	fmt.Println(formatted)
	const wantPrefix = "l3: l2: l1: base"
	if !strings.HasPrefix(formatted, wantPrefix) {
		t.Fatalf("prefix = %q, want start with %q\n%s", formatted, wantPrefix, formatted)
	}

	firstLine := formatted
	if before, _, ok := strings.Cut(formatted, "\n"); ok {
		firstLine = before
	}
	if firstLine != wantPrefix {
		t.Fatalf("first line = %q, want %q\n%s", firstLine, wantPrefix, formatted)
	}
}
