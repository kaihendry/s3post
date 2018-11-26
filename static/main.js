function fileSelected () {
  const BUCKET = document.getElementById('BUCKET').innerHTML
  const UPLOAD_ID = document.getElementById('UPLOAD_ID').innerHTML
  const REGION = document.getElementById('REGION').innerHTML
  const Policy = document.getElementById('Policy').innerHTML
  const Signature = document.getElementById('Signature').innerHTML

  const f = document.getElementById('file')
  const file = f.files[0]
  let key

  if (file) {
    const ymd = new Date().toISOString().slice(0, 10)
    key = ymd + '/' + file.name

    // filename can be overidden by the form input
    const filename = document.getElementById('filename').value
    if (filename) {
      key = ymd + '/' + filename + '.' + file.name.split('.').pop()
    }

    var fileSize = 0
    if (file.size > 1024 * 1024) fileSize = (Math.round(file.size * 100 / (1024 * 1024)) / 100).toString() + 'MB'; else fileSize = (Math.round(file.size * 100 / 1024) / 100).toString() + 'KB'
  }

  const fd = new window.FormData()

  fd.append('AWSAccessKeyId', UPLOAD_ID)
  fd.append('policy', window.btoa(Policy))
  fd.append('signature', Signature)

  fd.append('key', key)
  fd.append('acl', 'public-read')
  fd.append('Content-Type', file.type)
  fd.append('file', f.files[0])

  // Fetch doesn't support progress events yet
  // TODO: How to prevent browser from breaking upload whilst still in progress!
  window.fetch(`https://s3-${REGION}.amazonaws.com/${BUCKET}`, { method: 'POST', body: fd }).then(function (res) {
    if (res.ok) {
      document.getElementById('fileName').innerHTML = `<a href="//${BUCKET}/${key}">Name: ${key}</a>`
      document.getElementById('fileSize').innerHTML = `Size: ${fileSize}`
      document.getElementById('fileType').innerHTML = `Type: ${file.type}`

      // Notify NOTIFY_TOPIC via SNS of a successful upload
      window.fetch('/notify', { method: 'POST',
        body: JSON.stringify({URL: `https://${BUCKET}/${key}`, Bucket: BUCKET, Key: key, ContentType: file.type})
      }).then(function (res) {
        if (res.ok) {
          console.log(res)
        } else {
          console.error(res)
        }
      })
    } else {
      console.error(res)
    }
  })

  return false
}
