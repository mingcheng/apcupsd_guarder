package main

import (
	"testing"
)

func Test_runScript(t *testing.T) {
	type args struct {
		path string
		arg  interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		{"echo", args{"/bin/echo", "ok!!"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := runScript(tt.args.path, tt.args.arg); err != nil {
				t.Fatalf("cmd.Run() failed with %s", err)
			}
		})
	}
}
