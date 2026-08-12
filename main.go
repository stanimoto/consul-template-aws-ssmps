package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"
)

const version = "0.1.2"

type ssmClient interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	GetParameters(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error)
}

func main() {
	if !validateArgs(os.Args) {
		os.Exit(1)
	}

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithEC2IMDSRegion())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %s", err)
	}

	optFns := []func(*ssm.Options){}
	if endpoint := os.Getenv("SSMPS_AWS_SSM_ENDPOINT"); endpoint != "" {
		optFns = append(optFns, func(o *ssm.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}
	svc := ssm.NewFromConfig(cfg, optFns...)

	basePath := os.Getenv("SSMPS_BASE_PATH")
	names := os.Args[1:]

	nameToPath := makeNameToPathMap(basePath, names)
	paths := []string{}
	for _, path := range nameToPath {
		paths = append(paths, path)
	}

	pathToValue, err := getMultipleParamValues(ctx, svc, paths)
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

func getParamValue(ctx context.Context, svc ssmClient, name string) (string, error) {
	output, err := svc.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: aws.Bool(true),
	})

	if err != nil {
		var pnf *types.ParameterNotFound
		var pvnf *types.ParameterVersionNotFound
		switch {
		case errors.As(err, &pnf):
			log.Printf("ssmps(%q) returned no data: %s", name, pnf.ErrorCode())
			return "", nil
		case errors.As(err, &pvnf):
			log.Printf("ssmps(%q) returned no data: %s", name, pvnf.ErrorCode())
			return "", nil
		default:
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				return "", fmt.Errorf("ssmps(%q) returned error: %s", name, apiErr.ErrorCode())
			}
			return "", fmt.Errorf("ssmps(%q) returned unknown error: %v", name, err)
		}
	}

	return *output.Parameter.Value, nil
}

func getMultipleParamValues(ctx context.Context, svc ssmClient, names []string) (map[string]string, error) {
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
		output, err := svc.GetParameters(ctx, &ssm.GetParametersInput{
			Names:          batch,
			WithDecryption: aws.Bool(true),
		})

		if err != nil {
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				return nil, fmt.Errorf("ssmps returned error: %s, message: %s", apiErr.ErrorCode(), apiErr.ErrorMessage())
			}
			return nil, fmt.Errorf("ssmps returned unknown error: %s", err)
		}

		for _, p := range output.InvalidParameters {
			log.Printf("ssmps(%q) returned no data: Invalid Parameter", p)
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
