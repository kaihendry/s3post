STACK = s3post-sam
SAM_CLI_TELEMETRY = 0
AWS_PROFILE = mine
AWS_DEFAULT_REGION = ap-southeast-1

DOMAINNAME = up.dabase.com
ACMCERTIFICATEARN = arn:aws:acm:ap-southeast-1:407461997746:certificate/87b0fd84-fb44-4782-b7eb-d9c7f8714908

deploy:
	sam build
	sam deploy --no-progressbar --resolve-s3 \
		--stack-name $(STACK) \
		--parameter-overrides \
			DomainName=$(DOMAINNAME) \
			ACMCertificateArn=$(ACMCERTIFICATEARN) \
			Password=$(PASSWORD) \
			UploadId=$(UPLOAD_ID) \
			UploadSecret=$(UPLOAD_SECRET) \
		--no-confirm-changeset --no-fail-on-empty-changeset \
		--capabilities CAPABILITY_IAM
	@echo "CNAME: $$(AWS_PROFILE=$(AWS_PROFILE) aws cloudformation describe-stacks --stack-name $(STACK) --region $(AWS_DEFAULT_REGION) --query 'Stacks[0].Outputs[?OutputKey==`CNAME`].OutputValue' --output text)"

build-MainFunction:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $(ARTIFACTS_DIR)/bootstrap .

build-TranscodeFunction:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $(ARTIFACTS_DIR)/bootstrap ./functions/transcode/
	cp functions/transcode/bin/* $(ARTIFACTS_DIR)/

download-bins:
	mkdir -p functions/transcode/bin
	# jpegtran + libjpeg.so.62 (arm64) from libjpeg-turbo
	curl -Lo /tmp/ljt.deb https://github.com/libjpeg-turbo/libjpeg-turbo/releases/download/3.1.4.1/libjpeg-turbo-official_3.1.4.1_arm64.deb
	cd /tmp && mkdir -p ljt && ar x ljt.deb && tar xf data.tar.xz -C ljt/
	cp /tmp/ljt/opt/libjpeg-turbo/bin/jpegtran functions/transcode/bin/
	cp /tmp/ljt/opt/libjpeg-turbo/lib64/libjpeg.so.62 functions/transcode/bin/
	# cwebp (arm64, static) from Google
	curl -Lo /tmp/libwebp.tar.gz https://storage.googleapis.com/downloads.webmproject.org/releases/webp/libwebp-1.6.0-linux-aarch64.tar.gz
	tar xf /tmp/libwebp.tar.gz -C /tmp/
	cp /tmp/libwebp-1.6.0-linux-aarch64/bin/cwebp functions/transcode/bin/
	# ffmpeg (arm64, static) from johnvansickle.com
	curl -Lo /tmp/ffmpeg.tar.xz https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-arm64-static.tar.xz
	tar xf /tmp/ffmpeg.tar.xz -C /tmp/
	cp /tmp/ffmpeg-*-arm64-static/ffmpeg functions/transcode/bin/
	chmod +x functions/transcode/bin/*

validate:
	sam validate

destroy:
	aws cloudformation delete-stack --stack-name $(STACK)

logs:
	sam logs --stack-name $(STACK) --tail

clean:
	rm -rf .aws-sam
