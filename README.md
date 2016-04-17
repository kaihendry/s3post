# Authenticated AJAX POST to AWS S3 example 🙌

Is there a faster or more pleasant way to upload from a generic Web browser?

# FAQ

Q: Does it support **multiple** [file uploads](https://html.spec.whatwg.org/multipage/forms.html#file-upload-state-(type=file))?

A: No, because AWS POST API does not support multiple file uploads

# Setup

Create `.creds.ini` with

	id = 'awsid'
	secret = 'awssecret'
	bucket = 'yourbucket'
	region = 'ap-southeast-1'

Don't forget to hide it from being served !
