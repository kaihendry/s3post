<!DOCTYPE html>
<html>
<head>
<title>Upload to AWS S3</title>
<meta name=viewport content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<style>
/* http://stackoverflow.com/questions/36400558/ */
body { font-size: x-large; }
.inputs { display: flex; flex-direction: column; align-items: left; justify-content: left; }
label { padding: 1em; margin: 0.3em; border: thin solid black; border-top-right-radius: 1em; }
</style>

<?php
$creds = parse_ini_file(".creds.ini");

// Expire policy after an hour ... still not much help since it can be abused for an hour then
$expiry = str_replace('+00:00', '.000Z', date("c", time() + 60*60));

$policy = '{
"expiration": "' . $expiry . '",
"conditions": [
{ "acl": "public-read" },
["starts-with", "$key", "'.date("Y-m-d").'/"],
["starts-with", "$Content-Type", ""],
{ "bucket": "' . $creds["bucket"] . '" }
]
}';

$policy_b64 = base64_encode($policy);
$signature = base64_encode(hash_hmac('sha1', $policy_b64, $creds["secret"], true));
?>


<script>
function fileSelected(f) {
	var file = f.files[0];
	if (file) {
		var ymd = new Date().toISOString().slice(0, 10);

		if (file.name == "image.jpeg") {
			// For IOS to have a unique filename
			var key = ymd + '/' + file.name.substring(0, file.name.lastIndexOf(".")) + Math.round(new Date().getTime()/1000.0) + ".jpg";
		} else {
			var key = ymd + '/' + file.name;
		}

		filename = document.getElementById("filename").value;

		if (filename) {
			var key = ymd + '/' + filename + '.' + file.name.split('.').pop();
		}

		var fileSize = 0;
		if (file.size > 1024 * 1024)
			fileSize = (Math.round(file.size * 100 / (1024 * 1024)) / 100).toString() + 'MB';
		else
			fileSize = (Math.round(file.size * 100 / 1024) / 100).toString() + 'KB';

		document.getElementById('fileName').innerHTML = '<a href=http://<?=$creds["bucket"]?>/' + key + '>Name: ' + key + '</a>';
		document.getElementById('fileSize').innerHTML = 'Size: ' + fileSize;
		document.getElementById('fileType').innerHTML = 'Type: ' + file.type;
	}

	var fd = new FormData();

	fd.append('AWSAccessKeyId', '<?=$creds["awsid"]?>');
	fd.append('policy', '<?=$policy_b64?>')
	fd.append('signature','<?=$signature?>');

	fd.append('key', key);
	fd.append('acl', 'public-read');
	fd.append('Content-Type', file.type);
	fd.append("file", f.files[0]);
	
	var xhr = new XMLHttpRequest();
	xhr.upload.addEventListener("progress", uploadProgress, false);
	xhr.addEventListener("load", uploadComplete, false);
	xhr.addEventListener("error", uploadFailed, false);
	xhr.addEventListener("abort", uploadCanceled, false);
	xhr.open("POST", "https://s3-<?=$creds["region"]?>.amazonaws.com/<?=$creds["bucket"]?>");
	xhr.send(fd);
}

function uploadProgress(evt) {
	if (evt.lengthComputable) {
		var percentComplete = Math.round(evt.loaded * 100 / evt.total);
		document.getElementById('progressNumber').innerHTML = percentComplete.toString() + '%';
	}
	else {
		document.getElementById('progressNumber').innerHTML = 'unable to compute';
	}
}

function uploadComplete(evt) {
	/* This event is raised when the server send back a response */
	document.getElementById('response').innerHTML = evt.target.responseText;
}

function uploadFailed(evt) {
	alert("There was an error attempting to upload the file.");
}

function uploadCanceled(evt) {
	alert("The upload has been canceled by the user or the browser dropped the connection.");
}

</script>
</head>
<body>

<!--<pre>
<?=$policy?>
</pre>-->

<p id=response></p>

<div id="fileName"></div>
<div id="fileSize"></div>
<div id="fileType"></div>
<div id="progressNumber"></div>

<div class=inputs>
<label><strong>Optional:</strong> Upload file name <input type=text id=filename></label>
<label>Upload file <input type=file onchange="fileSelected(this);" name=upload></label>
</div>

</body>
</html>
