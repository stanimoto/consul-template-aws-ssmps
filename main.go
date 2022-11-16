package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

const version = "0.1.1"

func main() {
	if !validateArgs(os.Args) {
		os.Exit(1)
	}
	var config aws.Config
	endpoint := os.Getenv("LOCALSTACK_ENDPOINT")
	if (endpoint != "") {
		config = aws.Config{
			Region:           aws.String(os.Getenv("AWS_REGION")),
			Credentials:      credentials.NewStaticCredentials("test", "test", ""),
			S3ForcePathStyle: aws.Bool(true),
			Endpoint:         aws.String(endpoint),
		  }
	}
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: config,
	}))

	var awsRegion *string
	if aws.StringValue(sess.Config.Region) == "" {
		ec2Session := session.New()
		ec2Svc := ec2metadata.New(ec2Session)
		ec2Region, err := ec2Svc.Region()
		if err == nil {
			awsRegion = aws.String(ec2Region)
		}
	}

	svc := ssm.New(sess, &aws.Config{
		Region: awsRegion,
	})

	basePath := os.Getenv("SSMPS_BASE_PATH")
	names := os.Args[1:]

	nameToPath := makeNameToPathMap(basePath, names)
	paths := []string{}
	for _, path := range nameToPath {
		paths = append(paths, path)
	}

	pathToValue, err := getMultipleParamValues(svc, paths)
	if err != nil {
		log.Fatal(err)
	}

	if len(names) == 1 {
		path := nameToPath[names[0]]
		fmt.Println(pathToValue[path])
	} else {
		// Treat as multiple parameters even if the same name is given twice.
		nameToValue := map[string]string{}

		for name, path := range nameToPath {
			value, ok := pathToValue[path]
			if ok {
				nameToValue[name] = value
			} else {
				nameToValue[name] = ""
			}
		}

		data, err := json.Marshal(nameToValue)
		if err != nil {
			log.Fatalf("Error marshalling data: %s", err)
		}
		fmt.Printf("%s\n", data)
	}

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

func makeNameToPathMap(basePath string, names []string) map[string]string {
	m := map[string]string{}

	for _, name := range names {
		path := makePath(basePath, name)
		m[name] = path
	}

	return m
}

func makeBatches(values []string, batchSize int) ([][]string, error) {
	batches := [][]string{}

	if batchSize < 1 {
		return nil, fmt.Errorf("batchSize must be greater than 0")
	}

	for batchSize < len(values) {
		values, batches = values[batchSize:], append(batches, values[0:batchSize:batchSize])
	}
	batches = append(batches, values)

	return batches, nil
}

func getParamValue(svc ssmiface.SSMAPI, name string) (string, error) {
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

func getMultipleParamValues(svc ssmiface.SSMAPI, names []string) (map[string]string, error) {
	batches, err := makeBatches(names, 10) // This is the limit of the GetParameters endpoint.
	if err != nil {
		log.Fatalf("ssmps returned error: %s", err)
	}

	pathToValue := map[string]string{}
	// Defaults to an empty string
	for _, name := range names {
		pathToValue[name] = ""
	}

	for _, batch := range batches {
		paths := []*string{}
		for _, path := range batch {
			copy := path
			paths = append(paths, &copy)
		}

		output, err := svc.GetParameters(&ssm.GetParametersInput{
			Names:          paths,
			WithDecryption: aws.Bool(true),
		})

		if err != nil {
			aerr, ok := err.(awserr.Error)
			if ok {
				return nil, fmt.Errorf("ssmps returned error: %s message: %s", aerr.Code(), aerr.Message())
			}
			return nil, fmt.Errorf("ssmps returned unknown error: %s", err)
		}

		for _, p := range output.InvalidParameters {
			log.Printf("ssmps(%q) returned no data: Invalid Parameter", *p)
		}

		for _, p := range output.Parameters {
			path := *p.Name
			if p.Selector != nil {
				path += *p.Selector
			}
			pathToValue[path] = *p.Value
		}
	}

	return pathToValue, nil
}
