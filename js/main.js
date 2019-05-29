let _abort = new AbortController();

window.addEventListener('load', function(event) {
  _abort = new AbortController();
  fetch('/mboxMain', { signal: _abort.signal })
    .then(r => r.json())
    .then(j => { updateMails(j); })
    .catch(err => { console.error(err); alert(err); });
});

function mboxClicked(server, username, mboxName, el) {
  _abort.abort();
  _abort = new AbortController();
  const currentMbox = document.getElementById('current-mbox');
  currentMbox.id = '';
  el.id = 'current-mbox';
  fetch('/mboxName?server='+encodeURI(server)+
    '&username='+encodeURI(username)+
    '&mboxName='+encodeURI(mboxName),
    { signal: _abort.signal })
    .then(r => r.json())
    .then(j => {
      updateMails(j);
      document.title = 'Egret - ' + mboxName;
    })
    .catch(err => { console.error(err); alert(err); });
}

function mailClicked(server, username, mboxName, uid) {
  fetch('/mail?server='+encodeURI(server)+
    '&username='+encodeURI(username)+
    '&mboxName='+encodeURI(mboxName)+
    '&uid='+encodeURI(uid))
    .then(r => r.json())
    .then(j => { updateMail(j); })
    .catch(err => { console.error(err); alert(err); });
}

function mailCloseClicked() {
  document.getElementById('mail').style.display = 'none';
}

function settingsClicked() {
  document.getElementById('settings').style.display = 'block';
}

function settingsCloseClicked() {
  document.getElementById('settings').style.display = 'none';
}

function addAccountClicked() {
  const server = document.getElementById('addAccountServer');
  const username = document.getElementById('addAccountUsername');
  const password = document.getElementById('addAccountPassword');
  const body = JSON.stringify({ server: server.value, username: username.value, password: password.value });
  if (username != '' && password != '') {
    fetch('/addAccount', { method: 'post', body: body })
      .then(r => {
        if (!r.ok) {
          console.error(r.statusText);
          alert(r.statusText);
        } else {
          const tr = document.createElement('tr');
          tr.innerHTML = '<td class="account-row">'+username.value+'</td>'+
            '<td><span onclick="removeAccountClicked('+
              '\''+server.value+'\', \''+username.value+'\', this'+
            ')">&times;</span></td>';
          document.getElementById('accounts').appendChild(tr);
          server.value = '';
          username.value = '';
          password.value = '';
        }
      })
      .catch(err => { console.error(err); alert(err); });
  }
}

function removeAccountClicked(server, username, el) {
  el.parentNode.style.color = 'gray';
  const body = JSON.stringify({ server: server, username: username });
  fetch('/removeAccount', { method: 'post', body: body })
    .then(r => {
      if (!r.ok) {
        console.error(r.statusText);
        alert(r.statusText);
        el.parentNode.style.color = null;
      } else {
        el.parentNode.remove();
      }
    })
    .catch(err => {
      el.parentNode.style.color = null;
      console.log(err);
      alert(err);
    });
}

function updateMails(json) {
  const mails = document.getElementById('mails');
  mails.innerHTML = json.Previews.reduce((str, msg) => {
    str += '<tr onclick="mailClicked('+
        '\''+json.Server+'\', \''+json.Username+'\', \''+json.Mbox+
        '\', \''+msg.Uid+'\''+
      ')">'+
        '<td class="mail-date">'+msg.Date+'</td>'+
        '<td class="mail-preview">'+msg.Subject+
          ' <span style="color: gray;">'+msg.Preview+'</span></td>'+
      '</tr>';
    return str;
  }, '');
}

function updateMail(json) {
  console.log(json.Subject, json.RawBody); //*
  const mail = document.getElementById('mail');
  mail.innerHTML = '<div>'+
      '<strong>'+json.Subject+'</strong><span class="fr" onclick="mailCloseClicked()">&times;</span>'+
      '<div>'+json.RawBody+'</div>'+
    '</div>';
  mail.style.display = 'block';
}