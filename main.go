package main

import (
	"context"
	"encoding/hex"
	pb "main/v1"
	"os"
	"os/signal"
	"sync/atomic"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/pubsub"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"golang.org/x/exp/slices"
)

// This is the Bigquery row as a golang object, this is what the inserter takes in order to queue a row
// If you want to add a new column, step one is adding it to this struct and type
type Item struct {
	SpanID    string
	Timestamp int64
	Duration  int64
	Service   string
	Name      string
	TraceId   string
}

// This is simply to handle batching of those rows
type Rows struct {
	Spans []*Item
}

type Clients struct {
	BQClient  *bigquery.Client
	PubClient *pubsub.Client
}

// Limits adding spans to only relevant span names. You can add to this array to add additional spans
// Does not have to be exact match
var whitelistSpan []string = []string{"send", "process"}

func NewClients(ctx context.Context, projectID string) (*Clients, error) {
	bqClient, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	pubClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		bqClient.Close()
		return nil, err
	}
	return &Clients{
		BQClient:  bqClient,
		PubClient: pubClient,
	}, nil
}

func (c *Clients) pullSpans(ctx context.Context, projectID, subID string) error {
	sub := c.PubClient.Subscription(subID)

	sub.ReceiveSettings.Synchronous = false
	sub.ReceiveSettings.NumGoroutines = 16
	sub.ReceiveSettings.MaxOutstandingMessages = 50

	// Receive messages for 10 seconds, which simplifies testing.
	// Comment this out in production, since `Receive` should
	// be used as a long running operation.
	// ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	// defer cancel()

	var received int32
	ExportTraceServiceRequest := &pb.ExportTraceServiceRequest{}
	log.Infof("Beginning to recieve Pubsub messages from subscription %s", subID)
	err := sub.Receive(ctx, func(_ context.Context, msg *pubsub.Message) {
		if msg.Attributes["ce-type"] == "org.opentelemetry.otlp.traces.v1" && msg.Data != nil {

			//deferred function to handle any panics from marshalling null data
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("Panic caught: %s", r)
					msg.Ack()
				}
			}()

			err := proto.Unmarshal(msg.Data, ExportTraceServiceRequest) //This is where protobuf is deserializing the binary data in the pubsub message and marshalling it into an object
			if err != nil {
				log.Print(err)
			}
			atomic.AddInt32(&received, 1)                                     //Count for possible monitoring
			for _, rs := range ExportTraceServiceRequest.GetResourceSpans() { //Iterate over ResourceSpans array in the object and send to the inserter
				if err := insertRows(c.BQClient, projectID, rs, ctx); err != nil {
					log.Error(err)
				}
			}
			msg.Ack()
		}
	})
	if err != nil {
		log.Errorf("sub.Receive: %s", err)
	}

	return nil
}

func insertRows(bqclient *bigquery.Client, projectID string, resourceSpans *v1.ResourceSpans, ctx context.Context) error {
	datasetID := os.Getenv("DATASETID")
	tableID := os.Getenv("TABLEID")

	inserter := bqclient.Dataset(datasetID).Table(tableID).Inserter() //Init Dataset inserter

	//Resource attributes marshalling into map for reference
	resourceAttrs := make(map[string]string)

	//Span attributes marshalling into map for reference
	spanAttrs := make(map[string]string)

	for _, resAtt := range resourceSpans.Resource.GetAttributes() {
		resourceAttrs[resAtt.GetKey()] = resAtt.GetValue().GetStringValue()
	}

	bqRows := &Rows{} //Init struct for Adding rows
	for _, ils := range resourceSpans.GetInstrumentationLibrarySpans() {
		for _, spans := range ils.GetSpans() {

			//This is where span attributes are being populated
			for _, spanAtt := range spans.GetAttributes() {
				spanAttrs[spanAtt.GetKey()] = spanAtt.GetValue().GetStringValue()
			}

			if slices.Contains(whitelistSpan, spans.Name) {
				//Item struct appended to Rows struct array. This is just to handle batching multiple spans in a single trace
				//Inside the struct the values for the specific span is calculated and saved
				//If you are adding a new column, this is step 2
				bqRows.Spans = append(bqRows.Spans, &Item{
					SpanID:    hex.EncodeToString(spans.GetSpanId()),
					Timestamp: int64(spans.GetStartTimeUnixNano()) / 1e6,                            //1e6 == convert ns to ms
					Duration:  int64(spans.GetEndTimeUnixNano()-spans.GetStartTimeUnixNano()) / 1e6, //convert ns to ms
					Service:   resourceAttrs["service.name"],
					Name:      spans.Name,
					TraceId:   spanAttrs["X-TraceId"],
				})
			}
		}
	}

	//This will iterate over the previously referenced Spans and insert the Row to BigQuery
	for _, row := range bqRows.Spans {
		if err := inserter.Put(ctx, row); err != nil {
			return err
		} else {
			log.Debugf("INFO: spanID %x sent to bigquery instance %s:%s:%s successfully\n", row.SpanID, projectID, datasetID, tableID)
		}
	}

	return nil
}

func main() {
	PROJECT := os.Getenv("PROJECT")
	SUBID := os.Getenv("SUBID")

	name, _ := os.Hostname()

	ctx := context.Background()
	c, err := NewClients(ctx, PROJECT)
	if err != nil {
		log.Fatalf("Failed to initialize clients: %v", err)
	}

	log.Infof("Starting %s...", name)
	log.Info("NOTE: Logs here will not show successful bigquery inputs. For that, change LOG_LEVEL to debug in the environment variables")
	err = c.pullSpans(ctx, PROJECT, SUBID)
	if err != nil {
		log.Error(err)
	}

	// Gracefully close BigQuery client on SIGINT
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		c.PubClient.Close()
		c.BQClient.Close()
		os.Exit(0)
	}()
}
