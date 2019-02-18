package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

func main() {
	if !validateArgs(os.Args) {
		os.Exit(1)
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := ssm.New(sess)

	fmt.Println(getMultipleParamValues(svc, os.Args[1:]))

	os.Exit(0)
}

func validateArgs(args []string) bool {
	if len(args) < 2 {
		log.Println("Too few arguments")
		return false
	}
	return true
}

func makePath(basePath string, paramName string) string {
	if strings.HasPrefix(paramName, "/") {
		return paramName
	}
	return normalizeBasePath(basePath) + normalizeParamName(paramName)
}

func normalizeBasePath(basePath string) string {
	if len(basePath) == 0 {
		return basePath
	}

	// Prepend slash
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	// Remove trailing slash
	basePath = strings.TrimRight(basePath, "/")

	return basePath
}

func normalizeParamName(name string) string {
	// Prepend slash
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}

	return name
}

func getParamValue(svc ssmiface.SSMAPI, name string) (string, error) {
	// TODO: use svc.GetParameters instead for optimization
	output, err := svc.GetParameter(&ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: aws.Bool(true),
	})

	if err != nil {
		aerr, ok := err.(awserr.Error)
		if ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound, ssm.ErrCodeParameterVersionNotFound:
				log.Printf("ssmps(%q) returned no data: %s", name, aerr.Code())
				return "", nil
			default:
				return "", fmt.Errorf("ssmps(%q) returned error: %s", name, aerr.Code())
			}
		} else {
			return "", fmt.Errorf("ssmps(%q) returned unknown error: %v", name, err)
		}
	}

	return *output.Parameter.Value, nil
}

func getMultipleParamValues(svc ssmiface.SSMAPI, names []string) string {
	m := map[string]string{}

	for _, name := range names {
		path := makePath(os.Getenv("SSMPS_BASE_PATH"), name)
		value, err := getParamValue(svc, path)
		if err != nil {
			log.Fatal(err)
		}
		m[name] = value
	}

	if len(m) == 1 {
		return m[names[0]]
	}

	data, err := json.Marshal(m)
	if err != nil {
		log.Fatalf("Error marshalling data: %s", err)
	}
	return fmt.Sprintf("%s", data)
}
