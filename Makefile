GOPATH_DIR := $(GOPATH)/src/github.com/open-telemetry/opentelemetry-proto
GENDIR := gen
GOPATH_GENDIR := $(GOPATH_DIR)/$(GENDIR)

OTEL_DOCKER_PROTOBUF ?= otel/build-protobuf:0.9.0
BUF_DOCKER ?= bufbuild/buf:1.7.0

## Test environment variables
export PROJECT := gcp_project
export SUBID := otel_to_bq-test
export DATASETID := otel_spans_nonprod
export TABLEID := testspans2
export LOG_LEVEL := INFO

## Requires Docker

.PHONY: update-schema
update-schema:
	if ! [ -d "./opentelemetry-proto" ]; then git clone https://github.com/open-telemetry/opentelemetry-proto.git; fi
	mkdir -p ./opentelemetry-proto/pb
	docker run --rm -u 502 -v${PWD}/opentelemetry-proto:${PWD}/opentelemetry-proto -w${PWD}/opentelemetry-proto ${OTEL_DOCKER_PROTOBUF} --proto_path=${PWD}/opentelemetry-proto --go_out=plugins=grpc:./pb opentelemetry/proto/collector/trace/v1/trace_service.proto
	mv -f opentelemetry-proto/pb/go.opentelemetry.io/proto/otlp/collector/trace/v1 .

test:
	docker run --rm  -v${PWD}:${PWD} -w${PWD} -v$(HOME)/.config/gcloud/application_default_credentials.json:/application_default_credentials.json \
	-e PROJECT=${PROJECT} -e SUBID=${SUBID} -eLOG_LEVEL=${LOG_LEVEL} -e DATASETID=${DATASETID} -e TABLEID=${TABLEID} -e GOOGLE_APPLICATION_CREDENTIALS=/application_default_credentials.json \
	golang:1.21.1-alpine3.18 go run main --no-cache

clean:
	rm -rf opentelemetry-proto/
