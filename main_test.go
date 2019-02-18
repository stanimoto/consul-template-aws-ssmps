package main

import (
	"errors"
	"reflect"
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

func (m *mockSSMClient) GetParameters(input *ssm.GetParametersInput) (*ssm.GetParametersOutput, error) {
	var err error
	output := &ssm.GetParametersOutput{}

	for _, name := range input.Names {
		switch *name {
		case "invalid":
			output.InvalidParameters = append(output.InvalidParameters, name)
		case "aws error":
			err = awserr.New(ssm.ErrCodeInvalidKeyId, "blah", nil)
		case "unknown error":
			err = errors.New("Unknown Error")
		default:
			nameAndSelector := strings.SplitN(*name, ":", 2)
			name := nameAndSelector[0]
			selector := ""
			if len(nameAndSelector) > 1 {
				selector = ":" + nameAndSelector[1]
			}
			param := &ssm.Parameter{
				Name:     &name,
				Selector: &selector,
				Value:    aws.String(strings.ToUpper(name + "&" + selector)),
			}
			output.Parameters = append(output.Parameters, param)
		}
	}

	return output, err
}

func TestGetMultipleParamValues(t *testing.T) {
	mockSvc := &mockSSMClient{}

	cases := []struct {
		args []string
		want map[string]string
	}{
		{[]string{"a"}, map[string]string{"a": "A&"}},
		{[]string{"a", "b"}, map[string]string{"a": "A&", "b": "B&"}},
		{[]string{"a:1"}, map[string]string{"a:1": "A&:1"}},
		{[]string{"a", "invalid"}, map[string]string{"a": "A&", "invalid": ""}},
	}

	for _, c := range cases {
		got, err := getMultipleParamValues(mockSvc, c.args)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("getMutipleParamValues(svc, %q) == %v, want %v", c.args, got, c.want)
		}
	}
}

func TestGetMultipleParamValuesError(t *testing.T) {
	mockSvc := &mockSSMClient{}

	cases := []struct {
		args []string
		want string
	}{
		{[]string{"aws error"}, "returned error"},
		{[]string{"unknown error"}, "returned unknown error"},
	}

	for _, c := range cases {
		_, err := getMultipleParamValues(mockSvc, c.args)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("getMutipleParamValues(svc, %q) == %v, want %v", c.args, err, c.want)
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

func TestMakeNameToPathMap(t *testing.T) {
	cases := []struct {
		basePath string
		names    []string
		want     map[string]string
	}{
		{"", []string{}, map[string]string{}},
		{"", []string{"p1", "p2"}, map[string]string{"p1": "/p1", "p2": "/p2"}},
		{"base", []string{"p1", "/p1", "p2", "/base/p2"}, map[string]string{"p1": "/base/p1", "/p1": "/p1", "p2": "/base/p2", "/base/p2": "/base/p2"}},
	}

	for _, c := range cases {
		got := makeNameToPathMap(c.basePath, c.names)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("makeNameToPathMap(%q, %q) == %q, want %q", c.basePath, c.names, got, c.want)
		}
	}
}

func TestMakeBatches(t *testing.T) {
	cases := []struct {
		inValues []string
		inSize   int
		want     [][]string
	}{
		{[]string{}, 1, [][]string{[]string{}}},
		{[]string{"a", "b"}, 1, [][]string{[]string{"a"}, []string{"b"}}},
		{[]string{"a", "b"}, 2, [][]string{[]string{"a", "b"}}},
		{[]string{"a", "b", "c"}, 2, [][]string{[]string{"a", "b"}, []string{"c"}}},
		{[]string{"a", "b", "c", "d"}, 2, [][]string{[]string{"a", "b"}, []string{"c", "d"}}},
	}

	for _, c := range cases {
		got, _ := makeBatches(c.inValues, c.inSize)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("makeBatches(%q, %d) == %q, want %q", c.inValues, c.inSize, got, c.want)
		}
	}
}

func TestMakeBatchesError(t *testing.T) {
	_, err := makeBatches([]string{}, 0)
	if err == nil {
		t.Errorf("makeBatches should fail when <1 for batchSize is given")
	}
}
