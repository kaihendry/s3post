function fileSelected () {
  const BUCKET = document.getElementById('BUCKET').innerHTML
  const AWS_ACCESS_KEY_ID = document.getElementById('AWS_ACCESS_KEY_ID').innerHTML
  const REGION = document.getElementById('REGION').innerHTML
  const Policy = document.getElementById('Policy').innerHTML
  const Signature = document.getElementById('Signature').innerHTML

  f = document.getElementById('file')
  var file = f.files[0]

  if (file) {
    var ymd = new Date().toISOString().slice(0, 10)
    var key = ymd + '/' + file.name

    // filename can be overidden by the form input
    filename = document.getElementById('filename').value
    if (filename) {
      var key = ymd + '/' + filename + '.' + file.name.split('.').pop()
    }

    var fileSize = 0
    if (file.size > 1024 * 1024) fileSize = (Math.round(file.size * 100 / (1024 * 1024)) / 100).toString() + 'MB'; else fileSize = (Math.round(file.size * 100 / 1024) / 100).toString() + 'KB'

    document.getElementById('fileName').innerHTML = `<a href="//${BUCKET}/${key}">Name: ${key}</a>`
    document.getElementById('fileSize').innerHTML = `Size: ${fileSize}`
    document.getElementById('fileType').innerHTML = `Type: ${file.type}`
  }

  var fd = new FormData()

  fd.append('AWSAccessKeyId', AWS_ACCESS_KEY_ID)
  fd.append('policy', window.btoa(Policy))
  fd.append('signature', Signature)

  fd.append('key', key)
  fd.append('acl', 'public-read')
  fd.append('Content-Type', file.type)
  fd.append('file', f.files[0])

  console.log(fd)

  fetch(`https://s3-${REGION}.amazonaws.com/${BUCKET}`, { method: 'POST', body: fd }).then(function (res) {
    if (res.ok) {
      var feedback = {}
      feedback['msg'] = `//${BUCKET}/${key}`
      fetch('https://eb1tv85d00.execute-api.ap-southeast-1.amazonaws.com/prod', { method: 'POST', body: JSON.stringify(feedback) }).then(function (res) {
        if (res.ok) {
          console.log(res)
        } else {
          console.log('error', res)
        }
      })
    } else {
      console.log('error', res)
    }
  })

  return false
}
