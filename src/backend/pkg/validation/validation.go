package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	MissingRequired = validationError("input can not be empty")
	TooMany         = validationError("too many values")
	ParseError      = validationError("could not parse input")
	LengthError     = validationError("unexpected input length")
)

func validationError(s string) error {
	return errors.New(s)
}

type ValidationIssues struct {
	issues map[string]error
}

func EmptyIssues() ValidationIssues {
	return ValidationIssues{
		issues: make(map[string]error, 0),
	}
}

func (v ValidationIssues) MarshalJSON() ([]byte, error) {
	m := make(map[string]string, len(v.issues))

	for key, err := range v.issues {
		var finalValue string
		if errors.Is(err, MissingRequired) {
			finalValue = MissingRequired.Error()
		} else if errors.Is(err, TooMany) {
			finalValue = TooMany.Error()
		} else if errors.Is(err, ParseError) {
			finalValue = ParseError.Error()
		} else if errors.Is(err, LengthError) {
			finalValue = LengthError.Error()
		} else {
			finalValue = ""
		}
		m[key] = finalValue
	}
	return json.Marshal(m)
}

func (v ValidationIssues) GetAll() map[string]error {
	return v.issues
}

func (v ValidationIssues) Error() error {
	if len(v.issues) == 0 {
		return nil
	}
	return UserError{
		Issues: v,
	}
}

func (v ValidationIssues) Set(k string, e error) {
	_, ok := v.issues[k]
	if ok {
		return
	}
	v.issues[k] = e
}

func (v ValidationIssues) Merge(other ValidationIssues) {
	m := v.issues
	for key, value := range other.issues {
		m[key] = value
	}
}

type UserError struct {
	Issues ValidationIssues
}

func (e UserError) Error() string {
	m := e.Issues.GetAll()
	var b strings.Builder

	fmt.Fprintf(&b, "validation issue with fields")
	i := 0
	for key := range m {
		fmt.Fprintf(&b, " '%s'", key)
		if i < len(m)-1 {
			fmt.Fprintf(&b, ",")
		}
		i++
	}
	return b.String()
}

func (e UserError) DetailedError() string {
	var b strings.Builder

	fmt.Fprintf(&b, "validation issue with fields")

	m := e.Issues.GetAll()
	i := 0
	for key := range m {
		fmt.Fprintf(&b, " '%s'", key)
		if i < len(m)-1 {
			fmt.Fprintf(&b, ",")
		}
		i++
	}
	return b.String()
}
