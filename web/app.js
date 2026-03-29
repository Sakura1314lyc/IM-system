const wsInput = document.getElementById('wsInput');
const connectBtn = document.getElementById('connectBtn');
const statusText = document.getElementById('statusText');
const usernameInput = document.getElementById('username');
const renameBtn = document.getElementById('renameBtn');
const refreshBtn = document.getElementById('refreshBtn');
const onlineList = document.getElementById('onlineList');
const messages = document.getElementById('messages');
const messageInput = document.getElementById('messageInput');
const sendBtn = document.getElementById('sendBtn');
const toUserInput = document.getElementById('toUser');
const privateRow = document.getElementById('privateRow');
const modeButtons = document.querySelectorAll('.mode');

let mode = 'public';
let sessionId = '';
let polling = false;
let fetchingUsers = false;

function apiBase() {
  const value = wsInput.value.trim();
  return value || window.location.origin;
}

function addMsg(text, kind = 'server') {
  const el = document.createElement('div');
  el.className = `msg ${kind}`;
  el.textContent = text;
  messages.appendChild(el);
  messages.scrollTop = messages.scrollHeight;
}

function setStatus(text, ok = false) {
  statusText.textContent = `状态：${text}`;
  statusText.style.color = ok ? 'var(--green)' : 'var(--danger)';
}

async function sendRaw(message) {
  if (!sessionId) {
    addMsg('未连接服务器，请先点击“连接服务”', 'error');
    return false;
  }
  const res = await fetch(`${apiBase()}/api/send?sessionId=${encodeURIComponent(sessionId)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message })
  });
  if (!res.ok) {
    addMsg('发送失败：会话已失效或服务不可用', 'error');
    return false;
  }
  return true;
}

async function pollLoop() {
  if (polling || !sessionId) return;
  polling = true;

  while (sessionId) {
    try {
      const res = await fetch(`${apiBase()}/api/poll?sessionId=${encodeURIComponent(sessionId)}`);
      if (!res.ok) {
        throw new Error('poll failed');
      }
      const data = await res.json();
      for (const msg of data.messages || []) {
        if (fetchingUsers && msg.includes('在线...')) {
          const m = msg.match(/\](.*?):在线\.\.\./);
          if (m && m[1]) {
            const li = document.createElement('li');
            li.textContent = m[1];
            onlineList.appendChild(li);
          }
          continue;
        }
        addMsg(msg, 'server');
      }
    } catch (err) {
      setStatus('连接中断');
      addMsg('轮询中断，请重新连接', 'error');
      sessionId = '';
      break;
    }
  }

  polling = false;
}

async function connect() {
  try {
    const res = await fetch(`${apiBase()}/api/connect`, { method: 'POST' });
    if (!res.ok) throw new Error('connect failed');
    const data = await res.json();
    sessionId = data.sessionId;
    setStatus('已连接', true);
    addMsg('已连接后端 TCP 服务', 'system');
    onlineList.innerHTML = '';
    await sendRaw('who');
    pollLoop();
  } catch (err) {
    setStatus('连接失败');
    addMsg('连接失败，请确认 web 网关和 TCP 服务已启动', 'error');
  }
}

connectBtn.addEventListener('click', connect);

modeButtons.forEach((btn) => {
  btn.addEventListener('click', () => {
    modeButtons.forEach((b) => b.classList.remove('active'));
    btn.classList.add('active');
    mode = btn.dataset.mode;
    privateRow.hidden = mode !== 'private';
    addMsg(`已切换到${mode === 'private' ? '私聊' : '公聊'}模式`, 'system');
  });
});

renameBtn.addEventListener('click', async () => {
  const name = usernameInput.value.trim();
  if (!name) return addMsg('请输入昵称', 'error');
  if (await sendRaw(`rename|${name}`)) {
    addMsg(`已发送改名请求：${name}`, 'self');
  }
});

refreshBtn.addEventListener('click', async () => {
  onlineList.innerHTML = '';
  fetchingUsers = true;
  if (await sendRaw('who')) {
    addMsg('正在拉取在线用户...', 'system');
    setTimeout(() => {
      fetchingUsers = false;
      if (!onlineList.children.length) {
        const li = document.createElement('li');
        li.textContent = '暂无在线用户或响应延迟';
        onlineList.appendChild(li);
      }
    }, 700);
  } else {
    fetchingUsers = false;
  }
});

sendBtn.addEventListener('click', async () => {
  const text = messageInput.value.trim();
  if (!text) return;

  if (mode === 'private') {
    const to = toUserInput.value.trim();
    if (!to) return addMsg('私聊模式请输入接收人', 'error');
    if (await sendRaw(`to|${to}|${text}`)) {
      addMsg(`我 -> ${to}: ${text}`, 'self');
      messageInput.value = '';
    }
    return;
  }

  if (await sendRaw(text)) {
    addMsg(`我: ${text}`, 'self');
    messageInput.value = '';
  }
});

window.addEventListener('beforeunload', async () => {
  if (!sessionId) return;
  await fetch(`${apiBase()}/api/close?sessionId=${encodeURIComponent(sessionId)}`, { method: 'POST' });
});

addMsg('先启动 Go 服务，再点击“连接服务”', 'system');
