package main

import (
	"reflect"
	"testing"
)

func TestParseCommand(t *testing.T) {
	testcases := []struct {
		line string
		cmd  command
	}{
		{
			"system hostname",
			command{
				Method: "system.hostname",
			},
		},
		{
			"object update subscriber 1234 foo=bar",
			command{
				Method: "object.update",
				Params: []interface{}{"subscriber", "1234", map[string]string{"foo": "bar"}},
			},
		},
		{
			"object update subscriber 1234 foo=bar,baz=quux",
			command{
				Method: "object.update",
				Params: []interface{}{"subscriber", "1234", map[string]string{"foo": "bar", "baz": "quux"}},
			},
		},
		{
			"object update subscriber 1234 foo=bar baz=quux",
			command{
				Method: "object.update",
				Params: []interface{}{"subscriber", "1234", map[string]string{"foo": "bar"}, map[string]string{"baz": "quux"}},
			},
		},
		{
			`object update subscriber 1234 {"foo":"bar","baz":"quux"}`,
			command{
				Method: "object.update",
				Params: []interface{}{"subscriber", "1234", map[string]interface{}{"foo": "bar", "baz": "quux"}},
			},
		},
		{
			`object update subscriber 1234 {"foo": "bar 1", "baz   2": "quux 2"}`,
			command{
				Method: "object.update",
				Params: []interface{}{"subscriber", "1234", map[string]interface{}{"foo": "bar 1", "baz   2": "quux 2"}},
			},
		},
	}

	for _, tc := range testcases {
		cmd, err := parseCommand(tc.line)
		if err != nil {
			t.Error(err)
			continue
		}
		if !reflect.DeepEqual(cmd, tc.cmd) {
			t.Errorf("Parse error;\n\t%#v\n\t%#v", cmd, tc.cmd)
		}
	}
}
