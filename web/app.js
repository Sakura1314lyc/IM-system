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

let mode = 'public';

modeButtons.forEach((button) => {
  button.addEventListener('click', () => {
    modeButtons.forEach((b) => b.classList.remove('active'));
    btn.classList.add('active');
    mode = btn.dataset.mode;
    privateRow.hidden = mode !== 'private';
    addMsg(`已切换到${mode === 'private' ? '私聊' : '公聊'}模式`, 'system');
  });
});

sendBtn.addEventListener('click', () => {
  const text = messageInput.value.trim();
  if (!text) return;

  const line = document.createElement('div');
  line.className = 'msg outgoing';

  if (mode === 'private') {
    const to = toUserInput.value.trim() || '未指定';
    line.textContent = `给 ${to}（私聊）：${text}`;
  } else {
    line.textContent = text;
  }

  messageList.appendChild(line);
  messageList.scrollTop = messageList.scrollHeight;
  messageInput.value = '';
});
