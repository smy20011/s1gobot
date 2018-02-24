package main

import (
	"testing"
)

type fakeSender struct {
	output []string
}

func (f *fakeSender) Send(message string) {
	f.output = append(f.output, message)
}

func TestDeploy(t *testing.T) {
	sender := fakeSender{}
	err := handleDeploy(&sender)
	if err != nil {
		t.Error(err)
	}
	for _, line := range sender.output {
		t.Log(line)
	}
}
