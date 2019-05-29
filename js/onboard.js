let _accounts = [];

function addAccountClicked() {
  const blocks = document.getElementById('settings-blocks');
  const template = document.getElementById('account-inputs');
  const accountInfo = template.content.cloneNode(true);
  blocks.appendChild(accountInfo);
}

function nextClicked() {
  const blocks = document.querySelectorAll('#settings-blocks .block');
  const accounts = [];
  for (const block of blocks) {
    const server = block.querySelector('input[name="server"]').value;
    const username = block.querySelector('input[name="username"]').value;
    const password = block.querySelector('input[name="password"]').value;
    if (server && server != "" && username && username != "" && password &&
      password != "")
    {
      accounts.push({ server: server, username: username, password: password });
    }
  }
  if (accounts.length > 0) {
    _accounts = accounts;
    document.getElementById('settings').style.display = 'none';
    document.getElementById('overview').style.display = 'inline-block';
    addAccountsToOverview();
  }
}

function cancelClicked() {
  document.getElementById('settings').style.display = 'inline-block';
  document.getElementById('overview').style.display = 'none';
  const blocks = document.querySelectorAll('#overview-blocks .block');
  for (const block of blocks) {
    block.remove();
  }
}

function finishClicked() {
  const body = JSON.stringify(_accounts);
  fetch('/onboard', { method: 'post', body: body })
    .then(r => {
      if (!r.ok) {
        console.error(r.statusText);
        alert(r.statusText);
      } else {
        location.reload(true);
      }
    })
    .catch(err => { console.error(err); alert(err); });
}

function addAccountsToOverview() {
  const overview = document.getElementById('overview-blocks');
  for (const account of _accounts) {
    const block = document.createElement('div');
    block.className = 'block';
    block.innerHTML = '<div>Server: '+account.server+'</div>'+
      '<div>Username: '+account.username+'</div>'+
      '<div>Password: '+asteriskify(account.password)+'</div>';
    overview.appendChild(block);
  }
}

function asteriskify(str) {
  return '*'.repeat(str.length);
}