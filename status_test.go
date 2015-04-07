package main

import (
	"reflect"
	"relay/smtpd"
	"testing"
)

func TestParseOutcomingError(t *testing.T) {

	output := ParseOutcomingError("450 TEST 123")
	input := smtpd.Error{Code: 450, Message: "TEST 123"}
	if !reflect.DeepEqual(input, output) {
		t.Errorf("expect '%s', got - '%s'", input.Error(), output.Error())
	}
	output = ParseOutcomingError("450")
	input = smtpd.Error{Code: 450, Message: ""}
	if !reflect.DeepEqual(input, output) {
		t.Errorf("expect '%s', got - '%s'", input.Error(), output.Error())
	}
	output = ParseOutcomingError("12")
	input = ErrMessageErrorUnknown
	if !reflect.DeepEqual(input, output) {
		t.Errorf("expect '%s', got - '%s'", input.Error(), output.Error())
	}
	output = ParseOutcomingError("relay access denied")
	input = smtpd.Error{Code: 450, Message: "relay access denied"}
	if !reflect.DeepEqual(input, output) {
		t.Errorf("expect '%s', got - '%s'", input.Error(), output.Error())
	}
}
