# Tools to backfill Prometheus remote storage

This repo contains two simple tools:

* `promdump` - dumps time series data from Prometheus server into JSON files.
* `promremotewrite` - sends data from JSON files to any service that implements
  Prometheus [remote write API](https://prometheus.io/docs/operating/integrations/#remote-endpoints-and-storage).

These tools can be used to backfill metric data from Prometheus into a
long-term storage system that implements the write API. JSON files are used
as intermediary storage format to allow manual changes to metric metadata
(metric name and labels).

## promdump

By default `promdump` issues a separate query for each 24 hours worth of data,
and writes each resulting batch of samples into a separate file. First is
necessary to avoid overloading Prometheus server with queries with very large
response size, second is to prevent `promremotewrite` from using too much RAM
(each JSON file needs to be loaded in memory).

If your metrics don't have much data, you can increase `-batch` and
`-batches_per_file` to avoid creating too many small files. If your metrics
have very high cardinality (lots of label values, resulting in many time
series per metric name), you might need to decrease `-batch` even further.

## Example

Dump all data of `node_filesystem_free` metric for the last year, issuing a
separate query for each 12hrs of data, storing 24hrs worth of data in a each
file:

    promdump -url=http://localhost:9090 \
      -metric='node_filesystem_free{job="node"}' \
      -out=fs_free.json -batch=12h -batches_per_file=2 -period=8760h

Read resulting files and write metric data into Influxdb, issuing up to 5
concurrent API requests:

    promremotewrite -concurrency=5 \
      -url='http://localhost:8086/api/v1/prom/write?db=prom fs_free.json.*
