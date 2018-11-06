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

    document.getElementById('fileName').innerHTML = `<a href="//${BUCKET}/${key}">Name: ${key}</a>`
    document.getElementById('fileSize').innerHTML = `Size: ${fileSize}`
    document.getElementById('fileType').innerHTML = `Type: ${file.type}`
  }

  const fd = new window.FormData()

  fd.append('AWSAccessKeyId', UPLOAD_ID)
  fd.append('policy', window.btoa(Policy))
  fd.append('signature', Signature)

  fd.append('key', key)
  fd.append('acl', 'public-read')
  fd.append('Content-Type', file.type)
  fd.append('file', f.files[0])

  window.fetch(`https://s3-${REGION}.amazonaws.com/${BUCKET}`, { method: 'POST', body: fd }).then(function (res) {
    if (res.ok) {
      // Notify me via SNS of a successful upload
      var feedback = {}
      feedback['msg'] = `//${BUCKET}/${key}`
      window.fetch('https://eb1tv85d00.execute-api.ap-southeast-1.amazonaws.com/prod', { method: 'POST', body: JSON.stringify(feedback) }).then(function (res) {
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
