### OTel Span protobuf deserializer to BigQuery

This program does the following:
1. Reads a OTel produced pub/sub message encoded in a protobuf binary format
2. Unmarshals it into a golang native data structure type
3. Sends it to a BigQuery dataset

This program was written to be deployed as a service reading from a pub/sub topic. This is necessary to gather span data in bigquery. 

The schema for the protobuf data structures are stored in the `/v1` folder

Resources and additional information:
- https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/googlecloudpubsubexporter
  
## Test

You can run a `make test` as long as you have docker installed locally. This will spin up a container that will run the bigquery service inserter against the "testrows" dataset. Make sure to check the subscription and other values and use `gcloud auth application-default login` if you're met with a permission denied

## Update Schema
There is a chance in the future that the protobuf schema is updated. If this happens, you can run a `make update-schema` using a image with protobuf installed. This will clone the repo, compile the protobuf to golang structs and automatically update it in the v1 folder. It should be compatible and standard, this can also be used to add any newly added functions in the future.
