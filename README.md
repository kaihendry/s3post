Video explaining the project: https://www.youtube.com/watch?v=75XYVGakPWI

# Authenticated AJAX POST to AWS S3 example 🙌

Requires the following environment variables to be set:

* BUCKET e.g. s.natalian.org, same domain as my CloudFront domain, https://s.natalian.org
* PASSWORD e.g. "abracadabra" used to authenticate the client via a cookie
* REGION e.g. ap-southeast-1, where the bucket is located
* UPLOAD_ID the AWS_ACCESS_KEY_ID for uploading to the bucket only
* UPLOAD_SECRET the secret AWS_SECRET_ACCESS_KEY counterpart to the restricted AWS_ACCESS_KEY_ID
* NOTIFY_TOPIC SNS topic that publishes the key of the upload

# S3 policy for restricting bucket upload

	{ "Version": "2012-10-17",
		"Statement": [
		{
			"Sid": "Stmt1460356082000",
			"Effect": "Allow",
			"Action": [
				"s3:Put*"
			],
			"Resource": [ "arn:aws:s3:::s.natalian.org", "arn:aws:s3:::s.natalian.org/*" ]
		}
		]
	}

TODO is to implement a native Golang SDK
[createPresignedPost](https://github.com/aws/aws-sdk-go-v2/issues/171)

https://github.com/TTLabs/EvaporateJS is far too complex. [Alex
Russell](https://twitter.com/slightlylate/status/1059599437998186498) says
[they are working on
it!](https://www.chromestatus.com/feature/5712608971718656)
