package errors

import (
	daverr "github.com/pkg/errors"
)

type transformationRequiredError interface {
	TransformationRequired() bool
}

type causer interface {
	Cause() error
}

type TransformationRequiredError struct{}

func (e TransformationRequiredError) Error() string {
	return "transformation required"
}
func (e TransformationRequiredError) TransformationRequired() bool {
	return true
}

func IsTransformationRequired(err error) bool {
	for err != nil {
		if tre, ok := err.(transformationRequiredError); ok {
			return tre.TransformationRequired()
		}

		c, ok := err.(causer)
		if !ok {
			return false
		}
		err = c.Cause()
	}
	return false
}

func New(s string) error {
	return daverr.New(s)
}

func Errorf(s string, args ...interface{}) error {
	return daverr.Errorf(s, args...)
}

func Wrap(err error, s string) error {
	return daverr.Wrap(err, s)
}

func Wrapf(err error, s string, args ...interface{}) error {
	return daverr.Wrapf(err, s, args...)
}
