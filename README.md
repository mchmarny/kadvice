# kadvice

Knative cluster events and metric streaming using Cloud Run, Cloud PubSub, Cloud Dataflow, and BigQuery ML. It comprises of two data flows:

1. Kubernetes validating webhook that sends pod events to Cloud Run-based preprocessing service. That service extracts portent elements and publishes processed event data to a Cloud PubSub topic. Finally, a Cloud Dataflow job drains the data on the topic into a BigQuery table.
2. Kubernetes-based application (POD) which polls GKE cluster API and extracts extracts runtime metrics (reserved and used CPU as well as reserved and used RAM) and then writes that data into BigQuery table

These two data sources combined with few SQL queries and BigQuery's ML model capabilities allow us to build potentially interesting recommendation services for Knative operation teams.

## Deployment

### Knative Events (#1)

The whole idea of using Knative service to monitor other Knative services brought up a few complications so to capture events from multiple Knative clusters we will use GCP Knative-compatible service called Cloud Run. The command to deploy a service into Cloud Run is basically the same to the one we would use to deploy into Cloud Run on GKE but without the `cluster` parameter.

```shell
gcloud beta run deploy kadvice \
		--image=gcr.io/cloudylabs-public/kadvice:0.1.12 \
		--region=us-central1
```

The command will return the Cloud Run provided URL. Let's capture it in an envirnemnt variable

```shell
SERVICE_URL=$(gcloud beta run services describe kadvice --format='value(domain)')
echo $SERVICE_URL
```

Now that we have our Cloud Run service deployed, we can configure the Knative webhook which will send events to the above deployed service. To do that we will use Kubernetes validating webhook.

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

The `kadvice` webhook will send all pod-level events to our service.

Next we are going to create the BigQuery dataset and table.

```shell
	bq mk kadvice
	bq query --use_legacy_sql=false "
	CREATE OR REPLACE TABLE kadvice.raw_events (
		event_id STRING NOT NULL,
		project STRING NOT NULL,
		cluster STRING NOT NULL,
		event_time TIMESTAMP NOT NULL,
		namespace STRING,
		name STRING,
		service STRING,
		revision STRING,
		configuration STRING,
		operation STRING,
		object_id STRING,
		object_kind STRING,
		object_creation_time TIMESTAMP,
		content BYTES
	)"
```

> Notice the `content` field, that will persist the original message sent in Kubernetes event so we can reprocess that data, if needed, in the future.

Once the table is created, we can create Cloud Dataflow job to drain topic to BigQuery.


```shell
PROJECT=$(gcloud config get-value project)
gcloud dataflow jobs run kadvice-topic-bq \
    	--gcs-location gs://dataflow-templates/latest/PubSub_to_BigQuery \
    	--parameters "inputTopic=projects/${PROJECT}/topics/kadvice,outputTableSpec=${PROJECT}:kadvice.raw_events"

```

Cloud Dataflow will take a couple of minutes to create the necessary resources. When done, you will see data in the `kadvice.raw_events` table in BigQuery.

### Knative Metrics (#2)

Similarly, to export GKE pod metric to BigQuery we will use the [kexport](https://github.com/mchmarny/kexport) service. This is a single container so we can deploy it with a single command to all the clusters we want to track.


```shell
PROJECT=$(gcloud config get-value project)
kubectl run kexport --env="INTERVAL=30s" \
	--replicas=1 --generator=run-pod/v1 \
	--image="gcr.io/cloudylabs-public/kexport:0.3.3"
```


## Data Analysis

Let's start by combining the two main Knative service-level events. When the revision is `created` and `deleted`. This will allow us to see the lifespan of the pod in seconds.

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

> Before you can save the above query as a view you will have to fully qualify the table. For example `kadvice.raw_events` will become `PROJECT_ID.kadvice.raw_events`

We can also combine the pod event data with it's runtime metrics which will give us the reserved vs used CPU and RAM metrics.

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

Finally, to pick up into the raw JSON of the original event, you can use the BigQuery build in functions. To extract single value:

```sql
SELECT JSON_EXTRACT(SAFE_CONVERT_BYTES_TO_STRING(content), "$.request['operation']")
FROM `cloudylabs.kadvice.raw_events`
```

And to print the entire message as string:

```sql
SELECT TO_JSON_STRING(SAFE_CONVERT_BYTES_TO_STRING(content), true)
FROM `cloudylabs.kadvice.raw_events`
```

## Recommendation

// TODO:



