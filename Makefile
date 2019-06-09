RELEASE=0.1.12
PROJECT=$(gcloud config get-value project)

.PHONY: clean

mod:
	go mod tidy
	go mod vendor

test:
	go test ./... -v

image: mod
	gcloud builds submit \
		--project cloudylabs-public \
		--tag gcr.io/cloudylabs-public/kadvice:$(RELEASE)

deploy:
	gcloud beta run deploy kadvice \
		--image=gcr.io/cloudylabs-public/kadvice:$(RELEASE) \
		--region=us-central1

service: image
	gcloud beta run deploy kadvice \
		--image=gcr.io/cloudylabs-public/kadvice:$(RELEASE) \
		--region=us-central1

serviceless:
	gcloud beta run services delete kadvice

schema:
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

job:
	gcloud dataflow jobs run kadvice-topic-bq \
    	--gcs-location gs://dataflow-templates/latest/PubSub_to_BigQuery \
    	--parameters "inputTopic=projects/${PROJECT}/topics/kadvice,outputTableSpec=${PROJECT}:kadvice.raw_events"