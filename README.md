# fake-lambda

Fake AWS Lambda service that allows the ability to tune the duration of a request 
and the error responses returned. This is the Lambda version of Fake Service 
[github.com/nicholasjackson/fake-service](github.com/nicholasjackson/fake-service)
and has the capability to act as an upstram or a downstream for Fake Service.



## Configuration

Configuration of Fake Lambda is done using environment variables, the following table shows 
the possible configuration options. Unlike Fake Service, Fake Lambda only has a subset of features
that are appropriate for serverless environments.

```
Configuration values are set using environment variables, for info please see the following list.

Environment variables:
  UPSTREAM_URIS  default: no default
       Comma separated URIs of the upstream services to call
  UPSTREAM_ALLOW_INSECURE  default: 'false'
       Allow calls to upstream servers, ignoring TLS certificate validation
  UPSTREAM_WORKERS  default: '1'
       Number of parallel workers for calling upstreams, defualt is 1 which is sequential operation
  UPSTREAM_REQUEST_BODY  default: no default
       Request body to send to send with upstream requests, NOTE: UPSTREAM_REQUEST_SIZE and UPSTREAM_REQUEST_VARIANCE are ignored if this is set
  UPSTREAM_REQUEST_SIZE  default: '0'
       Size of the randomly generated request body to send with upstream requests
  UPSTREAM_REQUEST_VARIANCE  default: '0'
       Percentage variance of the randomly generated request body
  MESSAGE  default: 'Hello World'
       Message to be returned from service
  NAME  default: 'Service'
       Name of the service
  HTTP_CLIENT_KEEP_ALIVES  default: 'false'
       Enable HTTP connection keep alives for upstream calls.
  HTTP_CLIENT_APPEND_REQUEST  default: 'true'
       When true the path, querystring, and any headers sent to the service will be appended to any upstream calls
  HTTP_CLIENT_REQUEST_TIMEOUT  default: '30s'
       Max time to wait before timeout for upstream requests, default 30s
  TIMING_50_PERCENTILE  default: '0s'
       Median duration for a request
  TIMING_90_PERCENTILE  default: '0s'
       90 percentile duration for a request, if no value is set, will use value from TIMING_50_PERCENTILE
  TIMING_99_PERCENTILE  default: '0s'
       99 percentile duration for a request, if no value is set, will use value from TIMING_90_PERCENTILE
  TIMING_VARIANCE  default: '0'
       Percentage variance for each request, every request will vary by a random amount to a maximum of a percentage of the total request time
  ERROR_RATE  default: '0'
       Decimal percentage of request where handler will report an error. e.g. 0.1 = 10% of all requests will result in an error
  ERROR_TYPE  default: 'http_error'
       Type of error [http_error, delay]
  ERROR_CODE  default: '500'
       Error code to return on error
  ERROR_DELAY  default: '0s'
       Error delay [1s,100ms]
  LOAD_CPU_ALLOCATED  default: '0'
       MHz of CPU allocated to the service, when specified, load percentage is a percentage of CPU allocated
  LOAD_CPU_CLOCK_SPEED  default: '1000'
       MHz of a Single logical core, default 1000Mhz
  LOAD_CPU_CORES  default: '-1'
       Number of cores to generate fake CPU load over, by default fake-service will use all cores
  LOAD_CPU_PERCENTAGE  default: '0'
       Percentage of CPU cores to consume as a percentage. I.e: 50, 50% load for LOAD_CPU_CORES. If LOAD_CPU_ALLOCATED is not specified CPU percentage is based on the Total CPU available
  LOAD_MEMORY_PER_REQUEST  default: '0'
       Memory in bytes consumed per request
  LOAD_MEMORY_VARIANCE  default: '0'
       Percentage variance of the memory consumed per request, i.e with a value of 50 = 50%, and given a LOAD_MEMORY_PER_REQUEST of 1024 bytes, actual consumption per request would be in the range 516 - 1540 bytes
  RAND_SEED  default: '1678959362'
       A seed to initialize the random number generators
```

## Building

To use Go with AWS Lambda you must provide a `linux` `amd64`  compiled binary for your Lambda deployment.
Use the following command to build the application.

```
  GOOS=linux GOARCH=amd64 go build -o ./bin/fake-lambda
```

## Deployments

An example deployment configuration with API Gateway for public access using Terraform 
can be found in the `./terraform folder`

First configure your authentication settings for the AWS API.

[https://registry.terraform.io/providers/hashicorp/aws/latest/docs#authentication-and-configuration](https://registry.terraform.io/providers/hashicorp/aws/latest/docs#authentication-and-configuration)

Then run `terraform init`

```
➜ terraform init

Initializing the backend...

Initializing provider plugins...
- Finding hashicorp/aws versions matching "~> 4.0"...
- Finding latest version of hashicorp/archive...
- Installing hashicorp/aws v4.58.0...
- Installed hashicorp/aws v4.58.0 (signed by HashiCorp)
- Installing hashicorp/archive v2.3.0...
- Installed hashicorp/archive v2.3.0 (signed by HashiCorp)

Terraform has created a lock file .terraform.lock.hcl to record the provider
selections it made above. Include this file in your version control repository
so that Terraform can guarantee to make the same selections by default when
you run "terraform init" in the future.

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
```

Then you can deploy the lambda

```
terraform apply

data.archive_file.lambda: Reading...
data.archive_file.lambda: Read complete after 0s [id=ba4d77ed078f110adaf54939b65fe23a07dc2986]
data.aws_iam_policy_document.assume_role: Reading...
data.aws_iam_policy_document.lambda_logging: Reading...
data.aws_iam_policy_document.lambda_logging: Read complete after 0s [id=4063422367]
data.aws_iam_policy_document.assume_role: Read complete after 0s [id=3693445097]

Terraform used the selected providers to generate the following execution plan. Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # aws_api_gateway_deployment.example will be created

#...
aws_api_gateway_integration_response.lambda: Creation complete after 1s [id=agir-qkbxpkjojg-e0lkxo-ANY-200]
aws_lambda_permission.apigw: Creation complete after 1s [id=AllowAPIGatewayInvoke]
aws_api_gateway_deployment.example: Creation complete after 1s [id=hm8kyb]

Apply complete! Resources: 13 added, 0 changed, 0 destroyed.

Outputs:

lambda_url = "https://qkbxpkjojg.execute-api.us-east-1.amazonaws.com/test/fake-lambda"
```

Once deployed you can test fake-lambda. The Terraform code configures Fake Lambda
with API gateway to return errors 20% of the time. 

```
➜ curl https://qkbxpkjojg.execute-api.us-east-1.amazonaws.com/test/fake-lambda | jq
{
  "name": "fake-lambda",
  "uri": "http://localhost",
  "type": "HTTP",
  "ip_addresses": [
    "169.254.76.1",
    "169.254.79.130"
  ],
  "start_time": "2023-03-16T10:04:34.722307",
  "end_time": "2023-03-16T10:04:34.822689",
  "duration": "100.382003ms",
  "body": "Hello World",
  "code": 200
}

fake-lambda/terraform on  main [!?] via 🐹 v1.19.2 using ☁️  default/cloud-security-day 
{
  "name": "fake-lambda",
  "uri": "http://localhost",
  "type": "HTTP",
  "ip_addresses": [
    "169.254.76.1",
    "169.254.79.130"
  ],
  "start_time": "2023-03-16T10:04:36.107798",
  "end_time": "2023-03-16T10:04:36.208211",
  "duration": "100.412322ms",
  "body": "Hello World",
  "code": 200
}

➜ curl https://qkbxpkjojg.execute-api.us-east-1.amazonaws.com/test/fake-lambda | jq
{
  "errorMessage": "expected status 200, got status 500",
  "errorType": "errorString"
}
```

Fake Lambda can also be used as an upstream or a downstream to fake service as the response
is fully compatible since they share the same code.

```
➜ curl localhost:9090
{
  "name": "Code",
  "uri": "/",
  "type": "HTTP",
  "ip_addresses": [
    "172.19.188.34"
  ],
  "start_time": "2023-03-16T13:55:16.505263",
  "end_time": "2023-03-16T13:55:16.815322",
  "duration": "310.059053ms",
  "body": "Hello World",
  "upstream_calls": {
    "https://xrtjdlp6q0.execute-api.us-east-1.amazonaws.com/test/fake-lambda": {
      "name": "fake-lambda",
      "uri": "https://xrtjdlp6q0.execute-api.us-east-1.amazonaws.com/test/fake-lambda",
      "type": "HTTP",
      "ip_addresses": [
        "169.254.76.1",
        "169.254.79.130"
      ],
      "start_time": "2023-03-16T13:55:16.795218",
      "end_time": "2023-03-16T13:55:16.895577",
      "duration": "100.359536ms",
      "headers": {
        "Content-Length": "250",
        "Content-Type": "application/json",
        "Date": "Thu, 16 Mar 2023 13:55:16 GMT",
        "Via": "1.1 07e10376c59acfaa599a46483fdfeb7c.cloudfront.net (CloudFront)",
        "X-Amz-Apigw-Id": "B4HmyFTtIAMF4dA=",
        "X-Amz-Cf-Id": "GQ2XHf0khnKGo43SOjKu6tbg1sLFi93OOUBVIdz1Wza8Hmj-sFv2wQ==",
        "X-Amz-Cf-Pop": "LHR61-P7",
        "X-Amzn-Requestid": "aee474c6-00b4-4b69-995c-3c6c40af1b40",
        "X-Amzn-Trace-Id": "Root=1-64131fc4-78bed02949029dee5283bcc4;Sampled=0",
        "X-Cache": "Miss from cloudfront"
      },
      "body": "Hello World",
      "code": 200
    }
  },
  "code": 200
}
```

Once done, you can clean up your deployment by using the `terraform destroy` command.