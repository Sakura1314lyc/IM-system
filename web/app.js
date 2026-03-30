// ==================== DOM 元素 ====================
const loginPanel = document.getElementById('loginPanel');
const chatPanel = document.getElementById('chatPanel');
const groupsPanel = document.getElementById('groupsPanel');
const settingsPanel = document.getElementById('settingsPanel');

const usernameInput = document.getElementById('username');
const passwordInput = document.getElementById('password');
const connectBtn = document.getElementById('connectBtn');
const logoutBtn = document.getElementById('logoutBtn');

const userAvatar = document.getElementById('userAvatar');
const userNameDisplay = document.getElementById('userNameDisplay');
const userStatus = document.getElementById('userStatus');

const onlineList = document.getElementById('onlineList');
const refreshBtn = document.getElementById('refreshBtn');
const messageList = document.getElementById('messageList');
const messageInput = document.getElementById('messageInput');
const sendBtn = document.getElementById('sendBtn');
const toUserInput = document.getElementById('toUser');
const toGroupInput = document.getElementById('toGroup');
const privateRow = document.getElementById('privateRow');
const groupRow = document.getElementById('groupRow');

const modeBtns = document.querySelectorAll('.mode-btn');
const chatTitle = document.getElementById('chatTitle');
const chatSubtitle = document.getElementById('chatSubtitle');

const showGroupsBtn = document.getElementById('showGroupsBtn');
const showSettingsBtn = document.getElementById('showSettingsBtn');
const createGroupBtn = document.getElementById('createGroupBtn');
const newGroupNameInput = document.getElementById('newGroupName');

const darkModeToggle = document.getElementById('darkModeToggle');
const soundToggle = document.getElementById('soundToggle');
const fontSizeSlider = document.getElementById('fontSizeSlider');
const fontSizeValue = document.getElementById('fontSizeValue');

// ==================== 全局变量 ====================
let mode = 'public';
let eventSource = null;
let currentUsername = '';
let currentPassword = '';
let currentAvatar = '';
let myGroups = [];

// ==================== 工具函数 ====================
function showPanel(panelName) {
  loginPanel.style.display = panelName === 'login' ? 'flex' : 'none';
  chatPanel.style.display = panelName === 'chat' ? 'flex' : 'none';
  groupsPanel.style.display = panelName === 'groups' ? 'flex' : 'none';
  settingsPanel.style.display = panelName === 'settings' ? 'flex' : 'none';
}

function addMsg(text, type = 'incoming') {
  const msgDiv = document.createElement('div');
  msgDiv.className = `msg ${type}`;
  msgDiv.textContent = text;
  messageList.appendChild(msgDiv);
  messageList.scrollTop = messageList.scrollHeight;
}

function updateTitle() {
  const titles = {
    public: { title: '💬 聊天室', subtitle: '所有人都能看到的公聊内容' },
    private: { title: '💌 私聊', subtitle: '选择一个用户进行私密对话' },
    group: { title: '👥 群聊', subtitle: '加入群组与伙伴们交流' }
  };
  chatTitle.textContent = titles[mode].title;
  chatSubtitle.textContent = titles[mode].subtitle;
}

function updateUIForMode() {
  privateRow.style.display = mode === 'private' ? 'flex' : 'none';
  groupRow.style.display = mode === 'group' ? 'flex' : 'none';
  
  if (mode === 'private') {
    toUserInput.placeholder = '输入用户名';
  } else if (mode === 'group') {
    toGroupInput.placeholder = '输入群名';
  }
  updateTitle();
}

// ==================== 事件监听 ====================

// 登录事件
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

// 连接到 SSE 服务
function connect() {
  if (eventSource) eventSource.close();
  
  eventSource = new EventSource(
    `/api/events?name=${encodeURIComponent(currentUsername)}&password=${encodeURIComponent(currentPassword)}`
  );

  eventSource.onmessage = (event) => {
    addMsg(event.data, 'incoming');
  };

  eventSource.addEventListener('system', (event) => {
    addMsg(event.data, 'system');
  });

  eventSource.onerror = () => {
    addMsg('连接断开', 'system');
    showPanel('login');
  };

  // 成功连接后，使用 /api/online 获取用户信息
  fetch('/api/online')
    .then(res => res.json())
    .then(data => {
      if (data.online && data.online.length > 0) {
        const user = data.online.find(u => u.name === currentUsername);
        if (user) {
          currentAvatar = user.avatar;
          userAvatar.textContent = user.avatar;
        } else {
          currentAvatar = '👤';
          userAvatar.textContent = '👤';
        }
      }
      userNameDisplay.textContent = currentUsername;
      userStatus.textContent = '🟢 在线';
      showPanel('chat');
      refreshOnlineList();
    })
    .catch(err => console.error('获取用户信息失败:', err));
}

// 模式切换
modeBtns.forEach(btn => {
  btn.addEventListener('click', () => {
    modeBtns.forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    mode = btn.dataset.mode;
    updateUIForMode();
  });
});

// 刷新在线用户列表
async function refreshOnlineList() {
  try {
    const response = await fetch('/api/online');
    const data = await response.json();
    onlineList.innerHTML = '';
    
    if (data.online && data.online.length > 0) {
      data.online.forEach(user => {
        const li = document.createElement('li');
        li.innerHTML = `<span>${user.avatar || '👤'}</span> ${user.name}`;
        li.title = user.name;
        li.addEventListener('click', () => {
          toUserInput.value = user.name;
          mode = 'private';
          modeBtns.forEach(b => b.classList.remove('active'));
          document.querySelector('[data-mode="private"]').classList.add('active');
          updateUIForMode();
        });
        onlineList.appendChild(li);
      });
    } else {
      const li = document.createElement('li');
      li.className = 'loading';
      li.textContent = '暂无用户在线';
      onlineList.appendChild(li);
    }
  } catch (err) {
    console.error('刷新在线列表失败:', err);
  }
}

refreshBtn.addEventListener('click', refreshOnlineList);

// 发送消息
sendBtn.addEventListener('click', sendMessage);
messageInput.addEventListener('keypress', (e) => {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    sendMessage();
  }
});

async function sendMessage() {
  const text = messageInput.value.trim();
  if (!text) return;

  const payload = {
    name: currentUsername,
    message: text,
    mode
  };

  if (mode === 'private') {
    payload.to = toUserInput.value.trim();
    if (!payload.to) {
      alert('请输入私聊用户名');
      return;
    }
  } else if (mode === 'group') {
    payload.to = toGroupInput.value.trim();
    if (!payload.to) {
      alert('请输入群名');
      return;
    }
  }

  try {
    const response = await fetch('/api/send', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });

    if (response.ok) {
      messageInput.value = '';
      let displayText = text;
      
      if (mode === 'private') {
        displayText = `[私聊] ${currentAvatar} 我 -> ${payload.to}: ${text}`;
        addMsg(displayText, 'private');
      } else if (mode === 'group') {
        displayText = `[群聊 ${payload.to}] ${currentAvatar} 我: ${text}`;
        addMsg(displayText, 'group');
      } else {
        displayText = `${currentAvatar} 我: ${text}`;
        addMsg(displayText, 'outgoing');
      }
    } else {
      const error = await response.text();
      alert('发送失败: ' + error);
    }
  } catch (err) {
    console.error('发送消息失败:', err);
    alert('发送失败');
  }
}

// 群组管理
showGroupsBtn.addEventListener('click', () => showPanel('groups'));
showSettingsBtn.addEventListener('click', () => showPanel('settings'));

createGroupBtn.addEventListener('click', async () => {
  const groupName = newGroupNameInput.value.trim();
  if (!groupName) {
    alert('请输入群组名称');
    return;
  }

  if (!myGroups.includes(groupName)) {
    myGroups.push(groupName);
    newGroupNameInput.value = '';
    updateGroupsList();
    addMsg(`群组 ${groupName} 创建成功`, 'system');
  } else {
    alert('群组已存在');
  }
});

function updateGroupsList() {
  const list = document.getElementById('myGroupsList');
  list.innerHTML = '';
  
  if (myGroups.length === 0) {
    const li = document.createElement('li');
    li.className = 'empty';
    li.textContent = '暂无群组';
    list.appendChild(li);
  } else {
    myGroups.forEach(groupName => {
      const li = document.createElement('li');
      li.innerHTML = `
        <span>👥 ${groupName}</span>
        <button class="btn btn-sm" onclick="joinGroup('${groupName}')">进入</button>
      `;
      list.appendChild(li);
    });
  }
}

function joinGroup(groupName) {
  toGroupInput.value = groupName;
  mode = 'group';
  modeBtns.forEach(b => b.classList.remove('active'));
  document.querySelector('[data-mode="group"]').classList.add('active');
  updateUIForMode();
  showPanel('chat');
}

// 退出登录
logoutBtn.addEventListener('click', () => {
  if (eventSource) eventSource.close();
  currentUsername = '';
  currentPassword = '';
  currentAvatar = '';
  myGroups = [];
  messageList.innerHTML = '';
  showPanel('login');
});

// 设置面板事件
darkModeToggle.addEventListener('change', () => {
  document.body.classList.toggle('dark-mode', darkModeToggle.checked);
  localStorage.setItem('darkMode', darkModeToggle.checked);
});

fontSizeSlider.addEventListener('input', (e) => {
  const size = e.target.value;
  fontSizeValue.textContent = size + 'px';
  document.body.style.fontSize = size + 'px';
  localStorage.setItem('fontSize', size);
});

// ==================== 初始化 ====================
function init() {
  // 恢复用户偏好设置
  const darkMode = localStorage.getItem('darkMode') === 'true';
  const fontSize = localStorage.getItem('fontSize') || '14';
  
  if (darkMode) {
    document.body.classList.add('dark-mode');
    darkModeToggle.checked = true;
  }
  
  fontSizeSlider.value = fontSize;
  document.body.style.fontSize = fontSize + 'px';
  fontSizeValue.textContent = fontSize + 'px';
  
  // 初始化 UI
  showPanel('login');
  updateUIForMode();
  
  // 添加欢迎消息
  addMsg('欢迎来到 IM System！请先登录。', 'system');
}

init();

