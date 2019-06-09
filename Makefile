.PHONY: clean

mod:
	go mod tidy
	go mod vendor

meta:
	PROJECT=$(gcloud config get-value project)
	PROJECT_NUM=$(gcloud projects list --filter="${PROJECT}" --format="value(PROJECT_NUMBER)")

test:
	go test ./... -v

image: mod
	gcloud builds submit \
		--project cloudylabs-public \
		--tag gcr.io/cloudylabs-public/kadvice:0.1.7

deploy:
	gcloud beta run deploy kadvice \
		--image=gcr.io/cloudylabs-public/kadvice:0.1.7 \
		--region=us-central1

service: image
	gcloud beta run deploy kadvice \
		--image=gcr.io/cloudylabs-public/kadvice:0.1.7 \
		--region=us-central1

serviceless:
	gcloud beta run services delete preprocessd

sa:
	gcloud iam service-accounts create preprocessdinvoker \
    	--display-name "PreProcess Cloud Run Service Invoker"

	gcloud beta run services add-iam-policy-binding preprocessd \
		--member=serviceAccount:preprocessdinvoker@cloudylabs.iam.gserviceaccount.com \
		--role=roles/run.invoker
