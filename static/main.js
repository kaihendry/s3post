document.addEventListener('DOMContentLoaded', function () {
  const uploadFile = document.getElementById('uploadFile')
  uploadFile.addEventListener('change', () => {
    document.getElementById('uploadButton').disabled = false
  })
  window.addEventListener('paste', e => {
    uploadFile.files = e.clipboardData.files
  })
})

function fileSelected(form) {
  form.uploadButton.disabled = true
  const f = document.getElementById('uploadFile')
  const file = f.files[0]
  if (!file) return false

  const ymd = new Date().toISOString().slice(0, 10)
  const filename = document.getElementById('filename').value
  const ext = file.name.split('.').pop()
  const key = filename ? `${ymd}/${filename}.${ext}` : `${ymd}/${file.name}`

  fetch(`/presign?key=${encodeURIComponent(key)}`)
    .then(r => {
      if (!r.ok) throw new Error('presign failed: ' + r.status)
      return r.json()
    })
    .then(({ url }) => {
      document.getElementById('result').textContent = 'Uploading...'
      return fetch(url, {
        method: 'PUT',
        body: file,
        headers: { 'Content-Type': file.type || 'application/octet-stream' },
      })
    })
    .then(res => {
      if (!res.ok) throw new Error('upload failed: ' + res.status)
      const publicURL = `https://${document.querySelector('code').textContent}/${key}`
      document.getElementById('result').innerHTML =
        `Uploaded: <a href="//${document.querySelector('code').textContent}/${key}">${key}</a> — processing in background, you will be emailed when done`
      return fetch('/notify', {
        method: 'POST',
        body: JSON.stringify({ URL: publicURL, Bucket: document.querySelector('code').textContent, Key: key, ContentType: file.type }),
      })
    })
    .then(res => { if (!res.ok) console.error('notify failed', res) })
    .catch(err => {
      document.getElementById('result').innerHTML = `Error: ${err.message}`
      form.uploadButton.disabled = false
    })

  return false
}
