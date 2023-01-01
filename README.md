# Consul Template plugin for AWS SSM Parameter Store

[![Build Status](https://app.travis-ci.com/stanimoto/consul-template-aws-ssmps.svg?branch=master)](https://app.travis-ci.com/stanimoto/consul-template-aws-ssmps)

## Installation

1. Move the binary into `$PATH`.

## Configuring AWS Region

If you want to set the AWS region via the environment variable, set the "AWS_REGION" to the region you want.

```shell
$ AWS_REGION=us-west-2 consul-template ...
```

## Usage

### Single Parameter Name

If the parameter does not exist at the specified path, an emtpy string will be returned.

```liquid
{{ plugin "ssmps" "/foo/bar" }}
```

If you would like to get a value for a specific version, you can append a colon to the parameter name followed by a version number you want. If a specified version does not exist, an empty string will be returned.

```liquid
{{ plugin "ssmps" "/foo/bar:3" }}
```

### Multiple Parameter Names

You can use the plugin multiple times in a single template, but the plugin is executed for each occurance so it can be slow and you may hit the API rate limit especially if you run consul-template in multiple instances. As of writing, the Parameter Store currently has 40 GET API (shared) requests/sec limit.

If you pass multiple parameter names, they will be loaded in a single execution so it will be faster. A returned value is a JSON and you can save it to a variable for later use.

```liquid
{{ $map := plugin "ssmps" "foo" "bar" "baz" | parseJSON }}
{{ index $map "foo" }}
{{ index $map "bar" }}
{{ index $map "baz" }}
```

### Base Path

If you store values for the same key under the different paths for each environment, you may want to use the same key in the template but have it look at the different path.

For example, you have /production/database/password and /development/database/password in the Parameter Store.

```shell
$ export SSMPS_BASE_PATH=/production
```

```liquid
{{ plugin "ssmps" "database/password" }}
```

By setting the environment variable "SSMPS_BASE_PATH", the plugin gets the value from "/production/database/password".

If you set the "SSMPS_BASE_PATH" environment variable, the specified path in the template will be treated as a relative path if it doesn't have a leading slash. The specified path starting with a slash in the template is always treated as an absolute path. So, if you use environment specific values with the SSMPS_BASE_PATH but you want to use some environment agnostic value, you can always specify an absolute path.

Please note that if you don't set the "SSMPS_BASE_PATH" but the specified path in the template doesn't have a leading slash, it will still be treated as an absolute path. So, please be careful if you set a base path while the existing template uses paths in a relative path format.

### Custom Endpoint

You can use the environment variable `SSMPS_AWS_SSM_ENDPOINT` to override the default endpoint used for AWS requests.