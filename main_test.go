package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

type mockSSMClient struct {
	ssmiface.SSMAPI
}

func (m *mockSSMClient) GetParameter(input *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
	var param *ssm.Parameter
	var err error

	switch *input.Name {
	case "no param found":
		param, err = nil, awserr.New(ssm.ErrCodeParameterNotFound, "blah", nil)
	case "no param version found":
		param, err = nil, awserr.New(ssm.ErrCodeParameterVersionNotFound, "blah", nil)
	case "aws error":
		param, err = nil, awserr.New(ssm.ErrCodeInvalidKeyId, "blah", nil)
	case "unknown error":
		param, err = nil, errors.New("Unknown Error")
	default:
		param, err = &ssm.Parameter{Value: aws.String(strings.ToUpper(*input.Name))}, nil
	}

	return &ssm.GetParameterOutput{Parameter: param}, err
}

func TestGetMultipleParamValues(t *testing.T) {
	mockSvc := &mockSSMClient{}

	cases := []struct {
		args []string
		want string
	}{
		{[]string{"/a"}, "/A"},
		{[]string{"/a", "/b"}, "{\"/a\":\"/A\",\"/b\":\"/B\"}"},
	}

	for _, c := range cases {
		got := getMultipleParamValues(mockSvc, c.args)
		if got != c.want {
			t.Errorf("getMutipleParamValues(svc, %q) == %v, want %v", c.args, got, c.want)
		}
	}
}

func TestGetParamValue(t *testing.T) {
	mockSvc := &mockSSMClient{}

	cases := []struct {
		in  string
		out string
		err error
	}{
		{"ok", "OK", nil},
		{"no param found", "", nil},
		{"no param version found", "", nil},
		{"aws error", "", errors.New(ssm.ErrCodeInvalidKeyId)},
		{"unknown error", "", errors.New("Unknown Error")},
	}

	for _, c := range cases {
		got, err := getParamValue(mockSvc, c.in)

		if c.err == nil {
			if !(got == c.out && err == nil) {
				t.Errorf("getParamValue(svc, %q) == (%q, %v), want (%q, %v)", c.in, got, err, c.out, c.err)
			}
		} else {
			// Expects an error
			if !(got == "" && err != nil && strings.Contains(err.Error(), c.err.Error())) {
				t.Errorf("getParamValue(svc, %q) == (%q, %v), want (%q, %v)", c.in, got, err, c.out, c.err)
			}
		}
	}

}

func TestValidateArgs(t *testing.T) {
	cases := []struct {
		in   []string
		want bool
	}{
		{[]string{"program"}, false},
		{[]string{"program", "foo"}, true},
		{[]string{"program", "foo", "bar"}, true},
	}

	for _, c := range cases {
		got := validateArgs(c.in)
		if got != c.want {
			t.Errorf("validateArgs(%q) == %v, want %v", c.in, got, c.want)
		}
	}
}

func TestMakePath(t *testing.T) {
	cases := []struct {
		inPrefix, inParamName, want string
	}{
		{"", "foo/bar", "/foo/bar"},
		{"", "/foo/bar", "/foo/bar"},
		{"/", "foo/bar", "/foo/bar"},
		{"/", "/foo/bar", "/foo/bar"},
		{"foo/bar", "baz", "/foo/bar/baz"},
		{"foo/bar/", "baz", "/foo/bar/baz"},
		{"foo/bar", "/baz", "/baz"},
		{"foo/bar/", "/baz", "/baz"},
	}

	for _, c := range cases {
		got := makePath(c.inPrefix, c.inParamName)
		if got != c.want {
			t.Errorf("makePath(%q, %q) == %q, want %q", c.inPrefix, c.inParamName, got, c.want)
		}
	}
}

func TestNormalizeBasePath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"/", ""},
		{"foo/bar", "/foo/bar"},
		{"foo/bar/", "/foo/bar"},
		{"foo/bar//", "/foo/bar"},
		{"/foo/bar", "/foo/bar"},
		{"//foo/bar", "//foo/bar"}, // no change as expected
	}

	for _, c := range cases {
		got := normalizeBasePath(c.in)
		if got != c.want {
			t.Errorf("normalizeBasePath(%q) == %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeParamName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "/"},
		{"/", "/"},
		{"foo/bar", "/foo/bar"},
		{"foo/bar/", "/foo/bar/"},
		{"foo/bar//", "/foo/bar//"},
		{"/foo/bar", "/foo/bar"},
		{"//foo/bar", "//foo/bar"}, // no change as expected
	}

	for _, c := range cases {
		got := normalizeParamName(c.in)
		if got != c.want {
			t.Errorf("normalizeParamName(%q) == %q, want %q", c.in, got, c.want)
		}
	}
}
