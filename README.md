# kadvice

Kubernetes events streaming using Cloud Run and BigQuery ML



# streams

IoT stream processing using Cloud Run, PubSub, BigQuery, and Dataflow

BigQuery has recently added support for SQL queries over PubSub topic (Cloud Dataflow SQL, alpha). As a result it is now much easier to build stream analytics systems.

## Data Source

To illustrate stream processing and some analytics in BigQuery we will use synthetic and pre-processed data output from the [preprocessd](https://github.com/mchmarny/preprocessd) example. The PubSub payload of that data looks like this:

```json
{
    "source_id": "client-1",
    "event_id": "eid-20f9b215-4920-439d-9577-561fe776af4d",
    "event_ts": "2019-06-03T17:22:38.351698Z",
    "label": "utilization",
    "mem_used": 65.4931640625,
    "cpu_used": 10.526315789473683,
    "load_1": 1.4,
    "load_5": 1.62,
    "load_15": 1.68,
    "random_metric": 4.988959069746996,
    "mem_load_bucket"; "medium",
    "cpu_load_bucket": "small",
    "util_bias": "ram",
    "load_trend": -1,
    "combined_util": 25.333333333
}
```

Unless you changed the default target topic when configuring [preprocessd](https://github.com/mchmarny/preprocessd), the resulting data will be published to `processedevents`.

## Data Processing

## Configuration

Throughout this example we are going to be using your Google Cloud `project ID` and `project number` constructs. This are unique to your GCP configuration so let's start by capturing these values so we can re-use them throughout this example:

```shell
PRJ=$(gcloud config get-value project)
PRJ_NUM=$(gcloud projects list --filter="${PRJ}" --format="value(PROJECT_NUMBER)")
```

We will also need to enable a few GCP APIs:

```shell
gcloud services enable dataflow.googleapis.com
gcloud services enable compute.googleapis.com
gcloud services enable stackdriver.googleapis.com
gcloud services enable logging.googleapis.com
gcloud services enable bigquerystorage.googleapis.com
gcloud services enable storage-api.googleapis.com
gcloud services enable bigquery-json.googleapis.com
gcloud services enable bigquerystorage.googleapis.com
gcloud services enable pubsub.googleapis.com
gcloud services enable cloudresourcemanager.googleapis.com
```

### Stream Data into BigQuery

Next we are going to stream the PubSub topic data into BigQuery table. First, create the table using following schema:

```shell
bq mk streams
bq query --use_legacy_sql=false "
  CREATE OR REPLACE TABLE streams.payload_raw (
    source_id STRING NOT NULL,
    event_id STRING NOT NULL,
    event_ts TIMESTAMP NOT NULL,
    label STRING NOT NULL,
    mem_used FLOAT64 NOT NULL,
    cpu_used FLOAT64 NOT NULL,
    load_1 FLOAT64 NOT NULL,
    load_5 FLOAT64 NOT NULL,
    load_15 FLOAT64 NOT NULL,
    random_metric FLOAT64 NOT NULL,
    mem_load_bucket STRING NOT NULL,
    cpu_load_bucket STRING NOT NULL,
    util_bias STRING NOT NULL,
    load_trend INT64 NOT NULL,
    combined_util FLOAT64 NOT NULL
)"
```

Once the table is created, we can create Cloud Dataflow job to drain topic to BigQuery

```shell
gcloud dataflow jobs run processedevents-topic-to-bq \
    --gcs-location gs://dataflow-templates/latest/PubSub_to_BigQuery \
    --parameters \
inputTopic=projects/$PRJ/topics/processedevents,\
outputTableSpec=$PRJ:streams.payload_raw
```

## Data Analysis


Extract single value from JSON

```sql
SELECT JSON_EXTRACT(SAFE_CONVERT_BYTES_TO_STRING(content), "$.request['operation']")
FROM `cloudylabs.kadvice.raw_events`
```

Print out the entire payload

```sql
SELECT TO_JSON_STRING(SAFE_CONVERT_BYTES_TO_STRING(content), true)
FROM `cloudylabs.kadvice.raw_events`
```



















### Register Schema

First, we need to register the PubSub message schema including the shape of our synthetic data held in the `data` field.

```shell
gcloud beta data-catalog entries update \
    --lookup-entry="pubsub.topic.${PRJ}.processedevents" \
    --schema-from-file=schema.yaml
```

> Note, currently supported types in payload are: BYTE, INT16, INT32, INT64, FLOAT, DOUBLE, BOOLEAN, STRING, DECIMAL

### Event Windowing



> You can learn more about windowing functions and streaming pipelines [here](https://cloud.google.com/dataflow/docs/guides/sql/streaming-pipeline-basics)


```sql
SELECT
    c.payload.label as metric,
    TUMBLE_START("INTERVAL 30 SECOND") AS period_start,
    MIN(c.payload.load_1) as load_min,
    MAX(c.payload.load_1) as load_max,
    AVG(c.payload.load_1) as load_avg
FROM pubsub.topic.cloudylabs.processedevents as c
GROUP BY
    c.payload.label,
    TUMBLE(c.payload.event_ts, "INTERVAL 30 SECOND")
```

And using [Tumbling windows function](https://cloud.google.com/dataflow/docs/guides/sql/streaming-pipeline-basics#tumbling-windows) to display the streamed data in non overlapping time interval.



### Select Events
c
```sql
SELECT *
FROM buttons.iotevents_10s c
ORDER BY c.period_start DESC
```

Should return

```shell
| Row | button_color | period_start            | click_sum |
| --- | ------------ | ----------------------- | --------- |
| 1   | white        | 2019-05-31 23:16:25 UTC | 14        |
| 2   | white        | 2019-05-31 23:13:20 UTC | 1         |
| 3   | green        | 2019-05-31 23:13:20 UTC | 1         |
| 4   | black        | 2019-05-31 23:13:15 UTC | 4         |
```

