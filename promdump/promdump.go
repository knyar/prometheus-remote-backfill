// promdump fetches time series points for a given metric from Prometheus server
// and saves them into a series of json files. Files contain a serialized list of
// SampleStream messages (see model/value.go).
// Gemerated files can later be read by promremotewrite which will write the
// points to a Prometheus remote storage.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"time"

	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

var (
	baseURL        = flag.String("url", "http://localhost:9090", "URL for Prometheus server API")
	endTime        = flag.String("timestamp", "", "timestamp to end querying at (RFC3339). Defaults to current time.")
	periodDur      = flag.Duration("period", 7*24*time.Hour, "time period to get data for (ending at --timestamp)")
	batchDur       = flag.Duration("batch", 24*time.Hour, "batch size: time period for each query to Prometheus server.")
	metric         = flag.String("metric", "", "metric to fetch (can include label values)")
	out            = flag.String("out", "", "output file prefix")
	batchesPerFile = flag.Uint("batches_per_file", 1, "batches per output file")
)

// dump a slice of SampleStream messages to a json file.
func writeFile(values *[]*model.SampleStream, fileNum uint) error {
	if len(*values) == 0 {
		return nil
	}
	filename := fmt.Sprintf("%s.%05d", *out, fileNum)
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, valuesJSON, 0644)
}

func main() {
	flag.Parse()

	if *metric == "" || *out == "" {
		log.Fatalln("Please specify --metric and --out")
	}

	if periodDur.Nanoseconds()%1e9 != 0 || batchDur.Nanoseconds()%1e9 != 0 {
		log.Fatalln("--period and --batch must not have fractional seconds")
	}
	if *batchDur > *periodDur {
		batchDur = periodDur
	}

	endTS := time.Now()
	if *endTime != "" {
		var err error
		endTS, err = time.Parse(time.RFC3339, *endTime)
		if err != nil {
			log.Fatal(err)
		}
	}

	beginTS := endTS.Add(-*periodDur)
	batches := uint(math.Ceil(periodDur.Seconds() / batchDur.Seconds()))

	log.Printf("Will query from %v to %v in %v batches\n", beginTS, endTS, batches)

	ctx := context.Background()
	client, err := api.NewClient(api.Config{Address: *baseURL})
	if err != nil {
		log.Fatal(err)
	}
	api := v1.NewAPI(client)

	values := make([]*model.SampleStream, 0, 0)
	fileNum := uint(0)
	for batch := uint(1); batch <= batches; batch++ {
		queryTS := beginTS.Add(*batchDur * time.Duration(batch))
		lookback := batchDur.Seconds()
		if queryTS.After(endTS) {
			lookback -= queryTS.Sub(endTS).Seconds()
			queryTS = endTS
		}

		query := fmt.Sprintf("%s[%ds]", *metric, int64(lookback))
		log.Printf("Querying %s at %v", query, queryTS)
		value, err := api.Query(ctx, query, queryTS)
                
		if err != nil {
			log.Fatal(err)
		}

		if value.Type() != model.ValMatrix {
			log.Fatalf("Expected matrix value type; got %v", value.Type())
		}
		// model/value.go says: type Matrix []*SampleStream
		values = append(values, value.(model.Matrix)...)

		if batch%*batchesPerFile == 0 {
			writeFile(&values, fileNum)
			values = make([]*model.SampleStream, 0, 0)
			fileNum++
		}
	}
	writeFile(&values, fileNum)
}
