const modeButtons = document.querySelectorAll('.mode');
const privateRow = document.getElementById('privateRow');
const messageList = document.getElementById('messageList');
const sendBtn = document.getElementById('sendBtn');
const messageInput = document.getElementById('messageInput');
const toUserInput = document.getElementById('toUser');
const usernameInput = document.getElementById('username');
const renameBtn = document.getElementById('renameBtn');
const refreshBtn = document.getElementById('refreshBtn');
const onlineList = document.getElementById('onlineList');

let mode = 'public';
let username = '';
let evtSource = null;

function addMessage(text, type = 'system') {
  const el = document.createElement('div');
  el.className = `msg ${type}`;

  // 添加可爱emoji
  const emojis = type === 'incoming' ? ['💕', '🌸', '✨', '💖', '🌟'] :
                 type === 'outgoing' ? ['💌', '🌈', '🎀', '💝', '⭐'] :
                 ['🎉', '🌼', '💫', '🎈', '🌻'];
  const emoji = emojis[Math.floor(Math.random() * emojis.length)];

  el.textContent = type === 'system' ? text : `${emoji} ${text}`;
  messageList.appendChild(el);
  messageList.scrollTop = messageList.scrollHeight;
}

function setUsername(name) {
  username = name.trim();
  if (!username) {
    addMessage('请输入用户名后点击“修改”。', 'system');
    return;
  }

  if (evtSource) {
    evtSource.close();
  }

  evtSource = new EventSource(`/api/events?name=${encodeURIComponent(username)}`);
  evtSource.onmessage = function (evt) {
    addMessage(evt.data, 'incoming');
  };
  evtSource.onerror = function () {
    addMessage('与服务器连接已断开，正在尝试重连...', 'system');
    setTimeout(() => setUsername(username), 2000);
  };

  addMessage(`已使用昵称“${username}”连接`, 'system');
  loadOnlineUsers();
}

function loadOnlineUsers() {
  fetch('/api/online')
    .then((res) => res.json())
    .then((data) => {
      onlineList.innerHTML = '';
      (data.online || []).forEach((name) => {
        const li = document.createElement('li');
        li.textContent = name;
        onlineList.appendChild(li);
      });
    })
    .catch(() => {
      addMessage('获取在线用户失败。', 'system');
    });
}

modeButtons.forEach((button) => {
  button.addEventListener('click', () => {
    modeButtons.forEach((b) => b.classList.remove('active'));
    button.classList.add('active');
    mode = button.dataset.mode;
    privateRow.hidden = mode !== 'private';
  });
});

sendBtn.addEventListener('click', () => {
  if (!username) {
    addMessage('请先设置昵称。', 'system');
    return;
  }

  const text = messageInput.value.trim();
  if (!text) return;

  const payload = {
    name: username,
    message: text,
    mode: mode,
  };

  if (mode === 'private') {
    const to = toUserInput.value.trim();
    if (!to) {
      addMessage('私聊模式请填写目标用户名。', 'system');
      return;
    }
    payload.to = to;
  }

  fetch('/api/send', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
    .then((res) => {
      if (!res.ok) {
        return res.text().then((msg) => Promise.reject(msg));
      }
      const display = mode === 'private' ? `你给 ${payload.to}：${text}` : `你：${text}`;
      addMessage(display, 'outgoing');
      messageInput.value = '';
      loadOnlineUsers();
    })
    .catch((err) => {
      addMessage(`发送失败：${err}`, 'system');
    });
});

renameBtn.addEventListener('click', () => {
  const newName = usernameInput.value.trim();
  if (!newName) {
    addMessage('请输入昵称。', 'system');
    return;
  }

  if (!username) {
    setUsername(newName);
    return;
  }

  fetch('/api/rename', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ old: username, new: newName }),
  })
    .then((res) => {
      if (!res.ok) {
        return res.text().then((msg) => Promise.reject(msg));
      }
      setUsername(newName);
    })
    .catch((err) => {
      addMessage(`改名失败：${err}`, 'system');
    });
});

refreshBtn.addEventListener('click', loadOnlineUsers);

// 启动默认昵称
setUsername('访客_' + Math.floor(Math.random() * 10000));

