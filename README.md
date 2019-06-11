# kadvice

Kubernetes events streaming using Cloud Run and BigQuery ML



# streams

IoT stream processing using Cloud Run, PubSub, BigQuery, and Dataflow

BigQuery has recently added support for SQL queries over PubSub topic (Cloud Dataflow SQL, alpha). As a result it is now much easier to build stream analytics systems.

## Config

Define `$SERVICE_URL`

```shell
export SERVICE_URL="https://kadvice-2gtouos2pq-uc.a.run.app/PROJECT/CLUSTER"
```

Example

```shell
export SERVICE_URL="https://kadvice-2gtouos2pq-uc.a.run.app/cloudylabs/cr"
```

## Data Source

Validating webhook


```shell
cat <<EOF | kubectl apply -f -
apiVersion: admissionregistration.k8s.io/v1beta1
kind: ValidatingWebhookConfiguration
metadata:
  name: kadvice
webhooks:
  - name: kadvice.demo.knative.tech
    namespaceSelector: {}
    rules:
      - apiGroups:
          - ""
        apiVersions:
          - v1
        operations:
          - "*"
        resources:
          - pods
    failurePolicy: Ignore
    clientConfig:
      url: "${SERVICE_URL}"
EOF
```

To illustrate stream processing and some analytics in BigQuery we will use synthetic and pre-processed data output from the [preprocessd](https://github.com/mchmarny/preprocessd) example. The PubSub payload of that data looks like this:

```json
{
    EventID: 7f198e39-8ac5-11e9-80fa-42010a8e00b4,
    Project: cloudylabs
    Cluster: cr
    EventTime: 2019-06-09 14:47:25.744039537 +0000 UTC
    Namespace: demo
    Name: klogo-kfmq5-deployment-7656dc7769-bf9zh
    Service: klogo
    Revision: klogo-kfmq5
    Configuration: klogo
    Operation: CREATE
    ObjectID: 7f196748-8ac5-11e9-80fa-42010a8e00b4
    ObjectKind: Pod
    ObjectCreationTime: 2019-06-09 14:47:25 +0000 UTC
    Content:[...]
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







```
SELECT
    cp.creation_time,
    dp.deletion_time,
    TIMESTAMP_DIFF(dp.deletion_time, cp.creation_time, SECOND) as life_time,
    cp.project,
    cp.cluster,
    cp.namespace,
    cp.service,
    cp.pod_name,
    cp.revision
FROM (
  SELECT
    e.event_time as creation_time,
    e.project,
    e.cluster,
    e.namespace,
    e.name as pod_name,
    e.service,
    e.revision
  FROM kadvice.raw_events e
  WHERE
    e.object_kind = 'Pod'
    AND e.operation = 'CREATE'
) as cp JOIN (
  SELECT
    MAX(e.event_time) as deletion_time,
    e.name as pod_name
  FROM kadvice.raw_events e
  WHERE
    e.object_kind = 'Pod'
    AND e.operation = 'DELETE'
  GROUP BY e.name
) as dp ON cp.pod_name = dp.pod_name
ORDER BY 1 desc
```

> Before you can save the above query as `pods` view you will have to fully qualify the table. For example `kadvice.raw_events` will become `PROJECT_ID.kadvice.raw_events`


You can then combine this data with the pod metrics from `kexport`

```sql
select
  p.project,
  p.cluster,
  p.namespace,
  p.service,
  p.pod_name,
  p.creation_time,
  p.deletion_time,
  p.life_time,
  ROUND(MAX(m.reserved_ram),2) as max_reserved_ram,
  ROUND(MAX(m.reserved_cpu),2) as max_reserved_cpu,
  ROUND(AVG(m.used_ram),2) as avg_used_ram,
  ROUND(AVG(m.used_cpu),2) as avg_used_cpu
from kadvice.pods p
inner join kadvice.metrics m on p.pod_name = m.pod
  and m.metric_time between p.creation_time and p.deletion_time
group by
  p.project,
  p.cluster,
  p.namespace,
  p.service,
  p.pod_name,
  p.creation_time,
  p.deletion_time,
  p.life_time
```

Results in

```shell
| project 	| cluster 	| namespace 	| service 	| pod_name 	| creation_time 	| deletion_time 	| life_time 	| max_reserved_ram 	| max_reserved_cpu 	| avg_used_ram 	| avg_used_cpu 	|
|------------	|---------	|-----------	|----------	|--------------------------------------------	|--------------------------------	|--------------------------------	|-----------	|------------------	|------------------	|--------------	|--------------	|
| cloudylabs 	| cr 	| demo 	| kdemo 	| kdemo-x6rfc-deployment-8fc5bfb9f-drrmc 	| 2019-06-11 15:56:00.733781 UTC 	| 2019-06-11 15:58:57.301936 UTC 	| 176 	| 0 	| 25 	| 13337258.67 	| 4.33 	|
| cloudylabs 	| cr 	| demo 	| kuser 	| kuser-klzg6-deployment-6f9dcf64b7-sfbhw 	| 2019-06-11 14:59:36.697574 UTC 	| 2019-06-11 15:01:42.6629 UTC 	| 125 	| 0 	| 25 	| 16313344 	| 5 	|
| cloudylabs 	| cr 	| demo 	| kdemo 	| kdemo-x6rfc-deployment-8fc5bfb9f-zxczf 	| 2019-06-11 15:14:00.548681 UTC 	| 2019-06-11 15:17:04.490807 UTC 	| 183 	| 0 	| 25 	| 14891690.67 	| 4.5 	|
| cloudylabs 	| cr 	| demo 	| kdemo 	| kdemo-x6rfc-deployment-8fc5bfb9f-k2grd 	| 2019-06-11 16:14:01.234284 UTC 	| 2019-06-11 16:15:58.795991 UTC 	| 117 	| 0 	| 25 	| 12593152 	| 3.75 	|
| cloudylabs 	| cr 	| demo 	| maxprime 	| maxprime-nwgnk-deployment-7567cd9ccc-9z9d9 	| 2019-06-11 14:59:22.920464 UTC 	| 2019-06-11 15:01:53.127756 UTC 	| 150 	| 0 	| 25 	| 14295040 	| 3.8 	|
| cloudylabs 	| cr 	| demo 	| kdemo 	| kdemo-x6rfc-deployment-8fc5bfb9f-xjqnl 	| 2019-06-11 15:28:01.041772 UTC 	| 2019-06-11 15:30:09.19335 UTC 	| 128 	| 0 	| 25 	| 11587584 	| 3.75 	|
| cloudylabs 	| cr 	| demo 	| kdemo 	| kdemo-x6rfc-deployment-8fc5bfb9f-s7gjb 	| 2019-06-11 13:14:00.507299 UTC 	| 2019-06-11 13:17:04.10136 UTC 	| 183 	| 0 	| 25 	| 13595306.67 	| 4.5 	|
| cloudylabs 	| cr 	| demo 	| kdemo 	| kdemo-x6rfc-deployment-8fc5bfb9f-lfw5s 	| 2019-06-11 14:14:00.291517 UTC 	| 2019-06-11 14:15:56.382867 UTC 	| 116 	| 0 	| 25 	| 12356608 	| 4 	|
```

