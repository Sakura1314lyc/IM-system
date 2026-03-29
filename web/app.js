const modeButtons = document.querySelectorAll('.mode');
const privateRow = document.getElementById('privateRow');
const messageList = document.getElementById('messageList');
const sendBtn = document.getElementById('sendBtn');
const messageInput = document.getElementById('messageInput');
const toUserInput = document.getElementById('toUser');

let mode = 'public';

modeButtons.forEach((button) => {
  button.addEventListener('click', () => {
    modeButtons.forEach((b) => b.classList.remove('active'));
    button.classList.add('active');
    mode = button.dataset.mode;
    privateRow.hidden = mode !== 'private';
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
