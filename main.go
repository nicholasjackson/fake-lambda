package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/env"
	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/errors"
	"github.com/nicholasjackson/fake-service/handlers"
	"github.com/nicholasjackson/fake-service/load"
	"github.com/nicholasjackson/fake-service/logging"
	"github.com/nicholasjackson/fake-service/response"
	"github.com/nicholasjackson/fake-service/timing"
)

var upstreamURIs = env.String("UPSTREAM_URIS", false, "", "Comma separated URIs of the upstream services to call")
var upstreamAllowInsecure = env.Bool("UPSTREAM_ALLOW_INSECURE", false, false, "Allow calls to upstream servers, ignoring TLS certificate validation")
var upstreamWorkers = env.Int("UPSTREAM_WORKERS", false, 1, "Number of parallel workers for calling upstreams, defualt is 1 which is sequential operation")

var upstreamRequestBody = env.String("UPSTREAM_REQUEST_BODY", false, "", "Request body to send to send with upstream requests, NOTE: UPSTREAM_REQUEST_SIZE and UPSTREAM_REQUEST_VARIANCE are ignored if this is set")
var upstreamRequestSize = env.Int("UPSTREAM_REQUEST_SIZE", false, 0, "Size of the randomly generated request body to send with upstream requests")
var upstreamRequestVariance = env.Int("UPSTREAM_REQUEST_VARIANCE", false, 0, "Percentage variance of the randomly generated request body")

var message = env.String("MESSAGE", false, "Hello World", "Message to be returned from service")
var name = env.String("NAME", false, "Service", "Name of the service")

// Upstream client configuration
var upstreamClientKeepAlives = env.Bool("HTTP_CLIENT_KEEP_ALIVES", false, false, "Enable HTTP connection keep alives for upstream calls.")
var upstreamAppendRequest = env.Bool("HTTP_CLIENT_APPEND_REQUEST", false, true, "When true the path, querystring, and any headers sent to the service will be appended to any upstream calls")
var upstreamRequestTimeout = env.Duration("HTTP_CLIENT_REQUEST_TIMEOUT", false, 30*time.Second, "Max time to wait before timeout for upstream requests, default 30s")

// Service timing
var timing50Percentile = env.Duration("TIMING_50_PERCENTILE", false, time.Duration(0*time.Millisecond), "Median duration for a request")
var timing90Percentile = env.Duration("TIMING_90_PERCENTILE", false, time.Duration(0*time.Millisecond), "90 percentile duration for a request, if no value is set, will use value from TIMING_50_PERCENTILE")
var timing99Percentile = env.Duration("TIMING_99_PERCENTILE", false, time.Duration(0*time.Millisecond), "99 percentile duration for a request, if no value is set, will use value from TIMING_90_PERCENTILE")
var timingVariance = env.Int("TIMING_VARIANCE", false, 0, "Percentage variance for each request, every request will vary by a random amount to a maximum of a percentage of the total request time")

// performance testing flags
// these flags allow the user to inject faults into the service for testing purposes
var errorRate = env.Float64("ERROR_RATE", false, 0.0, "Decimal percentage of request where handler will report an error. e.g. 0.1 = 10% of all requests will result in an error")
var errorType = env.String("ERROR_TYPE", false, "http_error", "Type of error [http_error, delay]")
var errorCode = env.Int("ERROR_CODE", false, http.StatusInternalServerError, "Error code to return on error")
var errorDelay = env.Duration("ERROR_DELAY", false, 0*time.Second, "Error delay [1s,100ms]")

// load generation
var loadCPUAllocated = env.Int("LOAD_CPU_ALLOCATED", false, 0, "MHz of CPU allocated to the service, when specified, load percentage is a percentage of CPU allocated")
var loadCPUClockSpeed = env.Int("LOAD_CPU_CLOCK_SPEED", false, 1000, "MHz of a Single logical core, default 1000Mhz")
var loadCPUCores = env.Int("LOAD_CPU_CORES", false, -1, "Number of cores to generate fake CPU load over, by default fake-service will use all cores")
var loadCPUPercentage = env.Float64("LOAD_CPU_PERCENTAGE", false, 0, "Percentage of CPU cores to consume as a percentage. I.e: 50, 50% load for LOAD_CPU_CORES. If LOAD_CPU_ALLOCATED is not specified CPU percentage is based on the Total CPU available")

var loadMemoryAllocated = env.Int("LOAD_MEMORY_PER_REQUEST", false, 0, "Memory in bytes consumed per request")
var loadMemoryVariance = env.Int("LOAD_MEMORY_VARIANCE", false, 0, "Percentage variance of the memory consumed per request, i.e with a value of 50 = 50%, and given a LOAD_MEMORY_PER_REQUEST of 1024 bytes, actual consumption per request would be in the range 516 - 1540 bytes")

var seed = env.Int("RAND_SEED", false, int(time.Now().Unix()), "A seed to initialize the random number generators")

var logger *logging.Logger
var fakeRequest *handlers.Request

func init() {
	env.Parse()

	metrics := &logging.NullMetrics{}

	logger = logging.NewLogger(metrics, hclog.Default(), nil)
	logger.Log().Info("Using seed", "seed", *seed)

	requestDuration := timing.NewRequestDuration(
		*timing50Percentile,
		*timing90Percentile,
		*timing99Percentile,
		*timingVariance,
	)

	// create the error injector
	errorInjector := errors.NewInjector(
		logger.Log().Named("error_injector"),
		*errorRate,
		*errorCode,
		*errorType,
		*errorDelay,
		0.0,
		http.StatusServiceUnavailable,
	)

	// create the load generator
	// get the total CPU amount
	// If original CPU percent is 10, however the service has only been allocated 10% of the available CPU then percent should be 1 as it is total of avaiable
	// Allocated Percentage = Allocated / (Max * Cores) * Percentage
	// 100 / (1000 * 10) * 10 = 1

	if *loadCPUCores == -1 {
		*loadCPUCores = runtime.NumCPU()
	}

	if *loadCPUAllocated != 0 {
		*loadCPUPercentage = float64(*loadCPUAllocated) / (float64(*loadCPUClockSpeed) * float64(*loadCPUCores)) * float64(*loadCPUPercentage)
	}

	// create a generator that will be used to create memory and CPU load per request
	generator := load.NewGenerator(*loadCPUCores, *loadCPUPercentage, *loadMemoryAllocated, *loadMemoryVariance, logger.Log().Named("load_generator"))
	requestGenerator := load.NewRequestGenerator(*upstreamRequestBody, *upstreamRequestSize, *upstreamRequestVariance, int64(*seed))

	// create the httpClient
	defaultClient := client.NewHTTP(*upstreamClientKeepAlives, *upstreamAppendRequest, *upstreamRequestTimeout, *upstreamAllowInsecure)

	// build the map of gRPCClients
	grpcClients := make(map[string]client.GRPC)
	for _, u := range tidyURIs(*upstreamURIs) {
		//strip the grpc:// from the uri
		u2 := strings.TrimPrefix(u, "grpc://")

		c, err := client.NewGRPC(u2, *upstreamRequestTimeout)
		if err != nil {
			logger.Log().Error("Error creating GRPC client", "error", err)
			os.Exit(1)
		}

		grpcClients[u] = c
	}

	fakeRequest = handlers.NewRequest(
		*name,
		*message,
		requestDuration,
		tidyURIs(*upstreamURIs),
		*upstreamWorkers,
		defaultClient,
		grpcClients,
		errorInjector,
		generator,
		logger,
		requestGenerator,
		false,
		nil,
	)
}

func LambdaHandler(req handlers.Request) (response.Response, error) {

	d, err := json.Marshal(req)
	if err != nil {
		logger.Log().Error("unable to unmarshal request", "error", err)
		return response.Response{}, err
	}

	// create a HTTP request from the lambda request
	r, err := http.NewRequest(http.MethodGet, "http://localhost", bytes.NewBuffer(d))
	if err != nil {
		logger.Log().Error("unable to create request", "error", err)
		return response.Response{}, err
	}

	// use the response recorder as a simple way to wrap the response
	rr := httptest.NewRecorder()

	// handle the call
	fakeRequest.ServeHTTP(rr, r)

	// decode the response
	resp := &response.Response{}
	err = json.Unmarshal(rr.Body.Bytes(), resp)
	if err != nil {
		logger.Log().Error("unable to unmarshal response", "error", err)
		return response.Response{}, err
	}

	var respError error
	if rr.Code != http.StatusOK {
		respError = fmt.Errorf("expected status 200, got status %d", rr.Code)
	}

	return *resp, respError
}

func main() {
	lambda.Start(LambdaHandler)
}

// tidyURIs splits the upstream URIs passed by environment variable and returns
// a sanitised slice
func tidyURIs(uris string) []string {
	resp := []string{}
	rawResp := strings.Split(uris, ",")

	for _, r := range rawResp {
		r = strings.Trim(r, " ")
		if r != "" {
			resp = append(resp, r)
		}
	}

	return resp
}
