const connectBtn = document.getElementById('connectBtn');
const usernameInput = document.getElementById('username');
const passwordInput = document.getElementById('password');
const refreshBtn = document.getElementById('refreshBtn');
const onlineList = document.getElementById('onlineList');
const messageList = document.getElementById('messageList');
const messageInput = document.getElementById('messageInput');
const sendBtn = document.getElementById('sendBtn');
const toUserInput = document.getElementById('toUser');
const privateRow = document.getElementById('privateRow');
const modeButtons = document.querySelectorAll('.mode');

let mode = 'public';
let eventSource = null;
let currentUsername = '';
let currentPassword = '';

// 切换模式
modeButtons.forEach((button) => {
  button.addEventListener('click', () => {
    modeButtons.forEach((b) => b.classList.remove('active'));
    button.classList.add('active');
    mode = button.dataset.mode;
    privateRow.hidden = mode !== 'private' && mode !== 'group';
    if (mode === 'group') {
      toUserInput.placeholder = '输入群名';
    } else {
      toUserInput.placeholder = '输入用户名';
    }
    addMsg(`已切换到${mode === 'private' ? '私聊' : mode === 'group' ? '群聊' : '公聊'}模式`, 'system');
  });
});

// 添加消息到列表
function addMsg(text, type = 'message') {
  const msgDiv = document.createElement('div');
  msgDiv.className = `msg ${type}`;
  msgDiv.textContent = text;
  messageList.appendChild(msgDiv);
  messageList.scrollTop = messageList.scrollHeight;
}

// 连接到服务器
function connect() {
  if (!currentUsername || !currentPassword) {
    addMsg('请先输入用户名/密码并点击连接', 'system');
    return;
  }
  if (eventSource) eventSource.close();
  eventSource = new EventSource(`/api/events?name=${encodeURIComponent(currentUsername)}&password=${encodeURIComponent(currentPassword)}`);

  eventSource.onmessage = (event) => {
    addMsg(event.data);
  };

  eventSource.addEventListener('system', (event) => {
    addMsg(event.data, 'system');
  });

  eventSource.onerror = () => {
    addMsg('连接断开', 'system');
  };

  addMsg('连接中...', 'system');
}

// 登录/连接
connectBtn.addEventListener('click', () => {
  const name = usernameInput.value.trim();
  const password = passwordInput.value.trim();
  if (!name || !password) {
    alert('用户名和密码不能为空');
    return;
  }
  currentUsername = name;
  currentPassword = password;
  connect();
});

// 刷新在线用户
refreshBtn.addEventListener('click', async () => {
  const response = await fetch('/api/online');
  const data = await response.json();
  onlineList.innerHTML = '';
  data.online.forEach(user => {
    const li = document.createElement('li');
    li.textContent = user;
    onlineList.appendChild(li);
  });
});

// 发送消息
sendBtn.addEventListener('click', async () => {
  const text = messageInput.value.trim();
  if (!text) return;

  const payload = { name: currentUsername, message: text, mode };
  if (mode === 'private' || mode === 'group') {
    payload.to = toUserInput.value.trim();
    if (!payload.to) {
      alert(`请输入${mode === 'group' ? '群名' : '用户名'}`);
      return;
    }
  }

  const response = await fetch('/api/send', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  });

  if (response.ok) {
    messageInput.value = '';
    if (mode === 'private') {
      addMsg(`[私聊发送] ${text}`, 'outgoing');
    } else if (mode === 'group') {
      addMsg(`[群聊 ${payload.to}] ${text}`, 'outgoing');
    } else {
      addMsg(text, 'outgoing');
    }
  } else {
    alert('发送失败');
  }
});

// 初始化回车处理
usernameInput.addEventListener('keypress', (e) => {
  if (e.key === 'Enter') {
    connectBtn.click();
  }
});
passwordInput.addEventListener('keypress', (e) => {
  if (e.key === 'Enter') {
    connectBtn.click();
  }
});

addMsg('请先输入用户名/密码并点击连接', 'system');
