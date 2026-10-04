package main

import (
	"errors"
	"io"
	"testing"

	"github.com/lucap9056/corvauth/server/internal/config"
)

func TestRunCommand_Errors(t *testing.T) {
	t.Setenv(config.EnvDatabaseURL, "")

	tests := []struct {
		args []string
		want error
	}{
		{[]string{"foo"}, errUsage},
		{[]string{"schema", "drop"}, errUsage},
		{[]string{"schema", "apply"}, errDatabaseURLRequired},
	}
	for _, tt := range tests {
		if err := runCommand(tt.args, io.Discard); !errors.Is(err, tt.want) {
			t.Errorf("runCommand(%q) = %v; want %v", tt.args, err, tt.want)
		}
	}
}
