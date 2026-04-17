// ==================== DOM 元素 ====================
const DOM = {
  // 面板
  loginPanel: document.getElementById('loginPanel'),
  chatPanel: document.getElementById('chatPanel'),
  groupsPanel: document.getElementById('groupsPanel'),
  settingsPanel: document.getElementById('settingsPanel'),
  
  // 登录表单
  usernameInput: document.getElementById('username'),
  passwordInput: document.getElementById('password'),
  connectBtn: document.getElementById('connectBtn'),
  loginForm: document.getElementById('loginForm'),
  
  // 用户信息
  userAvatar: document.getElementById('userAvatar'),
  userNameDisplay: document.getElementById('userNameDisplay'),
  userStatus: document.getElementById('userStatus'),
  
  // 聊天区域
  onlineList: document.getElementById('onlineList'),
  refreshBtn: document.getElementById('refreshBtn'),
  messageList: document.getElementById('messageList'),
  messageInput: document.getElementById('messageInput'),
  sendBtn: document.getElementById('sendBtn'),
  toUserInput: document.getElementById('toUser'),
  toGroupInput: document.getElementById('toGroup'),
  privateRow: document.getElementById('privateRow'),
  groupRow: document.getElementById('groupRow'),
  
  // 模式切换
  modeBtns: document.querySelectorAll('.mode-btn'),
  chatTitle: document.getElementById('chatTitle'),
  chatSubtitle: document.getElementById('chatSubtitle'),
  
  // 头像上传
  loginAvatarUpload: document.getElementById('loginAvatarUpload'),
  loginAvatarPreview: document.getElementById('loginAvatarPreview'),
  settingsAvatarUpload: document.getElementById('settingsAvatarUpload'),
  settingsAvatarPreview: document.getElementById('settingsAvatarPreview'),
  saveAvatarBtn: document.getElementById('saveAvatarBtn'),
  
  // 其他功能
  logoutBtn: document.getElementById('logoutBtn'),
  showGroupsBtn: document.getElementById('showGroupsBtn'),
  showSettingsBtn: document.getElementById('showSettingsBtn'),
  loadHistoryBtn: document.getElementById('loadHistoryBtn'),
  createGroupBtn: document.getElementById('createGroupBtn'),
  newGroupNameInput: document.getElementById('newGroupName'),
  
  // 设置面板
  darkModeToggle: document.getElementById('darkModeToggle'),
  soundToggle: document.getElementById('soundToggle'),
  fontSizeSlider: document.getElementById('fontSizeSlider'),
  fontSizeValue: document.getElementById('fontSizeValue')
};

// 创建注册按钮
const registerBtn = document.createElement('button');
registerBtn.id = 'registerBtn';
registerBtn.type = 'button';
registerBtn.className = 'btn btn-secondary btn-lg';
registerBtn.textContent = '📝 注册新用户';
registerBtn.style.marginTop = '10px';
registerBtn.style.width = '100%';
DOM.loginForm.appendChild(registerBtn);

// 别名以保持向后兼容
const loginPanel = DOM.loginPanel;
const chatPanel = DOM.chatPanel;
const groupsPanel = DOM.groupsPanel;
const settingsPanel = DOM.settingsPanel;
const usernameInput = DOM.usernameInput;
const passwordInput = DOM.passwordInput;
const connectBtn = DOM.connectBtn;
const logoutBtn = DOM.logoutBtn;
const userAvatar = DOM.userAvatar;
const userNameDisplay = DOM.userNameDisplay;
const userStatus = DOM.userStatus;
const onlineList = DOM.onlineList;
const refreshBtn = DOM.refreshBtn;
const messageList = DOM.messageList;
const messageInput = DOM.messageInput;
const sendBtn = DOM.sendBtn;
const toUserInput = DOM.toUserInput;
const toGroupInput = DOM.toGroupInput;
const privateRow = DOM.privateRow;
const groupRow = DOM.groupRow;
const modeBtns = DOM.modeBtns;
const chatTitle = DOM.chatTitle;
const chatSubtitle = DOM.chatSubtitle;
const loginAvatarUpload = DOM.loginAvatarUpload;
const loginAvatarPreview = DOM.loginAvatarPreview;
const settingsAvatarUpload = DOM.settingsAvatarUpload;
const settingsAvatarPreview = DOM.settingsAvatarPreview;
const saveAvatarBtn = DOM.saveAvatarBtn;
const showGroupsBtn = DOM.showGroupsBtn;
const showSettingsBtn = DOM.showSettingsBtn;
const loadHistoryBtn = DOM.loadHistoryBtn;
const createGroupBtn = DOM.createGroupBtn;
const newGroupNameInput = DOM.newGroupNameInput;
const darkModeToggle = DOM.darkModeToggle;
const soundToggle = DOM.soundToggle;
const fontSizeSlider = DOM.fontSizeSlider;
const fontSizeValue = DOM.fontSizeValue;

// ==================== 全局变量 ====================
let mode = 'public';
let eventSource = null;
let authToken = '';
let currentUsername = '';
let currentPassword = '';
let currentAvatar = '';
let uploadedAvatarBase64 = null;
let myGroups = [];
let isLoading = false;

// ==================== 工具函数 ====================

/**
 * 转义 HTML 特殊字符，防止 XSS 攻击
 */
function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

/**
 * 显示/隐藏面板
 */
function showPanel(panelName) {
  loginPanel.style.display = panelName === 'login' ? 'flex' : 'none';
  chatPanel.style.display = panelName === 'chat' ? 'flex' : 'none';
  groupsPanel.style.display = panelName === 'groups' ? 'flex' : 'none';
  settingsPanel.style.display = panelName === 'settings' ? 'flex' : 'none';
}

/**
 * 显示提示信息
 */
function showNotification(message, type = 'info') {
  // 记录到控制台
  if (type === 'error') {
    console.error(`[错误] ${message}`);
  } else if (type === 'warn') {
    console.warn(`[警告] ${message}`);
  } else {
    console.log(`[信息] ${message}`);
  }
  
  // 也显示为 alert（可后续改进为 toast 通知）
  if (type === 'error') {
    alert(`❌ 错误: ${message}`);
  } else if (type === 'warn') {
    alert(`⚠️ 警告: ${message}`);
  }
  // info 类型不显示 alert，只记录到控制台
}

/**
 * 渲染头像 HTML
 */
function renderAvatar(avatar) {
  if (!avatar) {
    return '<span>👤</span>';
  }
  if (avatar.startsWith('data:')) {
    return `<img src="${escapeHtml(avatar)}" style="width:100%;height:100%;border-radius:50%;object-fit:cover;" />`;
  }
  return `<span>${escapeHtml(avatar)}</span>`;
}

/**
 * 添加消息到聊天框
 */
function addMsg(text, type = 'incoming') {
  const msgDiv = document.createElement('div');
  msgDiv.className = `msg ${type}`;

  if (text.startsWith('[私聊]')) {
    msgDiv.classList.add('private');
  } else if (text.startsWith('[群聊') || text.includes('[群聊')) {
    msgDiv.classList.add('group');
  } else if (text.includes(`我:`) || text.includes(`我 ->`)) {
    msgDiv.classList.add('outgoing');
  }

  msgDiv.textContent = escapeHtml(text);
  messageList.appendChild(msgDiv);
  messageList.scrollTop = messageList.scrollHeight;
}

/**
 * 更新聊天标题
 */
function updateTitle() {
  const titles = {
    public: { title: '💬 聊天室', subtitle: '所有人都能看到的公聊内容' },
    private: { title: '💌 私聊', subtitle: '选择一个用户进行私密对话' },
    group: { title: '👥 群聊', subtitle: '加入群组与伙伴们交流' }
  };
  chatTitle.textContent = titles[mode].title;
  chatSubtitle.textContent = titles[mode].subtitle;
}

/**
 * 更新 UI 根据当前模式
 */
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

/**
 * 统一的 API 调用函数
 */
async function apiCall(endpoint, options = {}) {
  try {
    isLoading = true;
    
    // 正确构建 fetch 配置对象，避免属性覆盖
    const { method = 'GET', headers = {}, body, ...rest } = options;
    
    const config = {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...headers
      },
      ...rest
    };
    
    // 处理 body：如果是对象则转换为 JSON 字符串
    if (body !== undefined) {
      config.body = typeof body === 'object' ? JSON.stringify(body) : body;
    }
    
    console.log(`[API] ${method} ${endpoint}`, config.body ? JSON.parse(config.body) : '');
    
    const response = await fetch(endpoint, config);
    
    if (!response.ok) {
      const error = await response.text();
      console.error(`[API Error] ${response.status}: ${error}`);
      throw new Error(error || `HTTP ${response.status}`);
    }
    
    const contentType = response.headers.get('content-type');
    if (contentType && contentType.includes('application/json')) {
      const data = await response.json();
      console.log(`[API Response] ${endpoint}`, data);
      return data;
    }
    
    const text = await response.text();
    console.log(`[API Response] ${endpoint}`, text);
    return text;
  } catch (err) {
    showNotification(`API 错误: ${err.message}`, 'error');
    throw err;
  } finally {
    isLoading = false;
  }
}

/**
 * 设置用户头像显示
 */
function setUserAvatar(avatar) {
  currentAvatar = avatar;
  userAvatar.innerHTML = renderAvatar(avatar);
}

function getAvatarLabel(avatar) {
  if (!avatar) {
    return '👤';
  }
  if (avatar.startsWith('data:')) {
    return '👤';
  }
  return escapeHtml(avatar);
}

// ==================== 事件监听 ====================

// 登录事件
connectBtn.addEventListener('click', async () => {
  const name = usernameInput.value.trim();
  const password = passwordInput.value.trim();
  
  if (!name || !password) {
    showNotification('用户名和密码不能为空', 'warn');
    return;
  }
  
  if (name.length > 50 || password.length > 100) {
    showNotification('用户名或密码过长', 'warn');
    return;
  }
  
  currentUsername = name;
  currentPassword = password;
  await connect();
});

// 注册事件
registerBtn.addEventListener('click', async () => {
  const name = usernameInput.value.trim();
  const password = passwordInput.value.trim();
  
  if (!name || !password) {
    showNotification('用户名和密码不能为空', 'warn');
    return;
  }

  if (password.length < 6) {
    showNotification('密码至少需要6个字符', 'warn');
    return;
  }

  if (name.length > 50) {
    showNotification('用户名过长', 'warn');
    return;
  }
  
  try {
    const avatarToUse = uploadedAvatarBase64 || '👤';
    const response = await fetch('/api/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: name, password: password, avatar: avatarToUse })
    });

    if (!response.ok) {
      const error = await response.text();
      showNotification(`注册失败: ${error}`, 'error');
      return;
    }

    showNotification('注册成功！请使用您的凭据登录。', 'info');
    uploadedAvatarBase64 = null;
    loginAvatarPreview.innerHTML = '📷';
  } catch (err) {
    console.error('注册失败:', err);
    showNotification(`注册失败: ${err.message}`, 'error');
  }
});

// ==================== 头像文件上传处理 ====================
function handleAvatarFileSelect(fileInput, preview) {
  fileInput.addEventListener('change', (e) => {
    const file = e.target.files[0];
    if (!file) return;

    // 验证文件大小
    const maxSize = 2 * 1024 * 1024; // 2MB
    if (file.size > maxSize) {
      showNotification(`图片太大（${(file.size / 1024 / 1024).toFixed(2)}MB），请选择小于 2MB 的图片`, 'warn');
      fileInput.value = '';
      return;
    }

    // 验证文件类型
    const validTypes = ['image/jpeg', 'image/png', 'image/gif', 'image/webp'];
    if (!validTypes.includes(file.type)) {
      showNotification(`不支持的图片格式。支持的格式: JPG, PNG, GIF, WebP`, 'warn');
      fileInput.value = '';
      return;
    }

    const reader = new FileReader();
    reader.onload = (event) => {
      const base64Data = event.target.result;
      uploadedAvatarBase64 = base64Data;
      
      const img = document.createElement('img');
      img.src = base64Data;
      img.style.width = '100%';
      img.style.height = '100%';
      img.style.borderRadius = '50%';
      img.style.objectFit = 'cover';
      preview.innerHTML = '';
      preview.appendChild(img);
    };
    reader.onerror = () => {
      showNotification('读取文件失败', 'error');
      fileInput.value = '';
    };
    reader.readAsDataURL(file);
  });

  preview.addEventListener('click', () => {
    fileInput.click();
  });
}

handleAvatarFileSelect(loginAvatarUpload, loginAvatarPreview);
handleAvatarFileSelect(settingsAvatarUpload, settingsAvatarPreview);

saveAvatarBtn.addEventListener('click', async () => {
  if (!authToken) {
    showNotification('请先登录后再修改头像', 'warn');
    return;
  }

  if (!uploadedAvatarBase64) {
    showNotification('请先选择一个头像图片', 'warn');
    return;
  }

  try {
    await apiCall('/api/avatar', {
      method: 'POST',
      body: { token: authToken, avatar: uploadedAvatarBase64 }
    });

    setUserAvatar(uploadedAvatarBase64);
    uploadedAvatarBase64 = null;
    settingsAvatarPreview.innerHTML = '📷';
    addMsg('头像更新成功！', 'system');
  } catch (err) {
    showNotification(`更新头像失败: ${err.message}`, 'error');
  }
});

// 连接到 SSE 服务
async function connect() {
  if (eventSource) eventSource.close();

  try {
    console.log('开始登录...', { username: currentUsername });
    
    const result = await apiCall('/api/login', {
      method: 'POST',
      body: { username: currentUsername, password: currentPassword }
    });

    console.log('登录成功，收到响应:', result);
    
    if (!result || !result.token) {
      throw new Error('服务器响应异常：缺少 token');
    }
    
    authToken = result.token;
    setUserAvatar(result.avatar || '👤');

    console.log('正在连接到事件流...');
    
    eventSource = new EventSource(`/api/events?token=${encodeURIComponent(authToken)}`);

    eventSource.onopen = () => {
      console.log('SSE 连接已建立');
    };

    eventSource.onmessage = (event) => {
      console.log('收到公聊消息:', event.data);
      addMsg(event.data, 'incoming');
    };

    eventSource.addEventListener('system', (event) => {
      console.log('收到系统消息:', event.data);
      addMsg(event.data, 'system');
    });

    eventSource.onerror = (err) => {
      console.error('SSE 连接错误:', err);
      addMsg('连接断开，请重新登录', 'system');
      if (eventSource) eventSource.close();
      showPanel('login');
    };

    userNameDisplay.textContent = currentUsername;
    userStatus.textContent = '🟢 在线';
    showPanel('chat');
    
    console.log('正在加载在线用户列表和群组...');
    await refreshOnlineList();
    await refreshGroupList();
    await loadHistory('public');
    
    console.log('登录完成！');
  } catch (err) {
    console.error('登录失败:', err);
    showNotification(`登录失败: ${err.message}`, 'error');
  }
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
    const data = await apiCall('/api/online');
    onlineList.innerHTML = '';
    
    if (data.online && data.online.length > 0) {
      const onlineUsers = data.online.filter(user => user.name !== currentUsername);
      
      if (onlineUsers.length === 0) {
        const li = document.createElement('li');
        li.className = 'loading';
        li.textContent = '暂无其他用户在线';
        onlineList.appendChild(li);
        return;
      }
      
      onlineUsers.forEach(user => {
        const li = document.createElement('li');
        li.innerHTML = `${renderAvatar(user.avatar)} <span>${escapeHtml(user.name)}</span>`;
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
    showNotification('刷新在线列表失败', 'error');
  }
}

refreshBtn.addEventListener('click', refreshOnlineList);

// 验证消息内容
function validateMessage(text, mode) {
  if (!text) {
    return { valid: false, message: '消息不能为空' };
  }
  
  if (text.length > 10000) {
    return { valid: false, message: '消息过长（最多 10000 个字符）' };
  }
  
  if (mode === 'private' && !toUserInput.value.trim()) {
    return { valid: false, message: '请输入私聊用户名' };
  }
  
  if (mode === 'group' && !toGroupInput.value.trim()) {
    return { valid: false, message: '请输入群名' };
  }
  
  return { valid: true };
}

// 发送消息
sendBtn.addEventListener('click', sendMessage);
messageInput.addEventListener('keypress', (e) => {
  if (e.key === 'Enter' && !e.shiftKey && !isLoading) {
    e.preventDefault();
    sendMessage();
  }
});

async function sendMessage() {
  const text = messageInput.value.trim();
  const validation = validateMessage(text, mode);
  
  if (!validation.valid) {
    showNotification(validation.message, 'warn');
    return;
  }

  const payload = {
    token: authToken,
    name: currentUsername,
    message: text,
    mode
  };

  if (mode === 'private') {
    payload.to = toUserInput.value.trim();
  } else if (mode === 'group') {
    payload.to = toGroupInput.value.trim();
  }

  try {
    await apiCall('/api/send', {
      method: 'POST',
      body: payload
    });

    messageInput.value = '';
    let displayText = escapeHtml(text);
    
    const avatarLabel = getAvatarLabel(currentAvatar);
    if (mode === 'private') {
      displayText = `[私聊] ${avatarLabel} 我 -> ${escapeHtml(payload.to)}: ${displayText}`;
      addMsg(displayText, 'private');
    } else if (mode === 'group') {
      displayText = `[群聊 ${escapeHtml(payload.to)}] ${avatarLabel} 我: ${displayText}`;
      addMsg(displayText, 'group');
    } else {
      displayText = `${avatarLabel} 我: ${displayText}`;
      addMsg(displayText, 'outgoing');
    }
  } catch (err) {
    showNotification(`发送失败: ${err.message}`, 'error');
  }
}

// 群组管理
showGroupsBtn.addEventListener('click', async () => {
  await refreshGroupList();
  showPanel('groups');
});
showSettingsBtn.addEventListener('click', () => showPanel('settings'));

createGroupBtn.addEventListener('click', async () => {
  const groupName = newGroupNameInput.value.trim();
  
  if (!groupName) {
    showNotification('请输入群组名称', 'warn');
    return;
  }
  
  if (groupName.length > 100) {
    showNotification('群组名称过长', 'warn');
    return;
  }

  try {
    await apiCall('/api/group', {
      method: 'POST',
      body: { token: authToken, action: 'create', groupName }
    });

    newGroupNameInput.value = '';
    addMsg(`已创建群组：${escapeHtml(groupName)}`, 'system');
    await refreshGroupList();
  } catch (err) {
    showNotification(`创建群组失败: ${err.message}`, 'error');
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
      const escapedGroupName = escapeHtml(groupName);
      li.innerHTML = `
        <span>👥 ${escapedGroupName}</span>
        <div class="group-controls">
          <button class="btn btn-sm" onclick="selectGroup('${groupName}')">进入</button>
          <button class="btn btn-secondary btn-sm" onclick="leaveGroup('${groupName}')">离开</button>
        </div>
      `;
      list.appendChild(li);
    });
  }
}

async function selectGroup(groupName) {
  toGroupInput.value = groupName;
  mode = 'group';
  modeBtns.forEach(b => b.classList.remove('active'));
  document.querySelector('[data-mode="group"]').classList.add('active');
  updateUIForMode();
  showPanel('chat');
}

async function leaveGroup(groupName) {
  try {
    await apiCall('/api/group', {
      method: 'POST',
      body: { token: authToken, action: 'leave', groupName }
    });

    addMsg(`已离开群组：${escapeHtml(groupName)}`, 'system');
    await refreshGroupList();
  } catch (err) {
    showNotification(`离开群组失败: ${err.message}`, 'error');
  }
}

async function refreshGroupList() {
  if (!authToken) {
    return;
  }

  try {
    const data = await apiCall(`/api/groups?token=${encodeURIComponent(authToken)}`);
    myGroups = data.groups || [];
    updateGroupsList();
  } catch (err) {
    console.error('刷新群组列表失败:', err);
    showNotification('刷新群组列表失败', 'error');
  }
}

async function loadHistory(type, target) {
  if (!authToken) {
    showNotification('请先登录后再加载历史', 'warn');
    return;
  }

  let url = `/api/history?token=${encodeURIComponent(authToken)}&type=${encodeURIComponent(type)}`;
  
  if (type === 'group') {
    if (!target) {
      showNotification('请输入群名以加载群聊历史', 'warn');
      return;
    }
    url += `&group=${encodeURIComponent(target)}`;
  }
  
  if (type === 'private') {
    if (!target) {
      showNotification('请输入私聊对象以加载私聊历史', 'warn');
      return;
    }
    url += `&peer=${encodeURIComponent(target)}`;
  }

  try {
    const data = await apiCall(url);
    const history = data.history || [];
    messageList.innerHTML = '';

    if (history.length === 0) {
      addMsg('暂无历史消息。', 'system');
      return;
    }

    history.reverse().forEach(item => {
      let text = escapeHtml(item.content);
      if (item.type === 'group') {
        text = `[群聊 ${escapeHtml(item.group)}] ${escapeHtml(item.from)}: ${text}`;
      } else if (item.type === 'private') {
        text = `[私聊] ${escapeHtml(item.from)} -> ${escapeHtml(item.to)}: ${text}`;
      } else {
        text = `${escapeHtml(item.from)}: ${text}`;
      }
      addMsg(text, item.type === 'group' ? 'group' : item.type === 'private' ? 'private' : 'incoming');
    });
  } catch (err) {
    showNotification(`加载历史失败: ${err.message}`, 'error');
  }
}

loadHistoryBtn.addEventListener('click', () => {
  const target = mode === 'group' ? toGroupInput.value.trim() : mode === 'private' ? toUserInput.value.trim() : '';
  loadHistory(mode, target);
});

// 退出登录
logoutBtn.addEventListener('click', async () => {
  if (eventSource) eventSource.close();

  if (authToken) {
    try {
      await apiCall('/api/logout', {
        method: 'POST',
        body: { token: authToken }
      });
    } catch (err) {
      // 忽略登出错误
    }
  }

  authToken = '';
  currentUsername = '';
  currentPassword = '';
  currentAvatar = '';
  uploadedAvatarBase64 = null;
  myGroups = [];
  messageList.innerHTML = '';
  userNameDisplay.textContent = '未登录';
  userStatus.textContent = '离线';
  userAvatar.innerHTML = '👤';
  showPanel('login');
});

// 设置面板事件
darkModeToggle.addEventListener('change', () => {
  const isDarkMode = darkModeToggle.checked;
  document.body.classList.toggle('dark-mode', isDarkMode);
  localStorage.setItem('darkMode', isDarkMode);
  addMsg(isDarkMode ? '已启用深色模式' : '已禁用深色模式', 'system');
});

soundToggle.addEventListener('change', () => {
  const isSoundEnabled = soundToggle.checked;
  localStorage.setItem('soundEnabled', isSoundEnabled);
  addMsg(isSoundEnabled ? '已启用声音提示' : '已禁用声音提示', 'system');
});

fontSizeSlider.addEventListener('input', (e) => {
  const size = parseInt(e.target.value);
  if (size < 12 || size > 24) {
    return;
  }
  fontSizeValue.textContent = size + 'px';
  document.body.style.fontSize = size + 'px';
  localStorage.setItem('fontSize', size);
});

// ==================== 初始化 ====================
function init() {
  // 恢复用户偏好设置
  const darkMode = localStorage.getItem('darkMode') === 'true';
  const fontSize = parseInt(localStorage.getItem('fontSize') || '14');
  const soundEnabled = localStorage.getItem('soundEnabled') !== 'false';
  
  if (darkMode) {
    document.body.classList.add('dark-mode');
    darkModeToggle.checked = true;
  }
  
  if (!soundEnabled) {
    soundToggle.checked = false;
  }
  
  fontSizeSlider.value = fontSize;
  fontSizeSlider.min = '12';
  fontSizeSlider.max = '24';
  document.body.style.fontSize = fontSize + 'px';
  fontSizeValue.textContent = fontSize + 'px';
  
  // 初始化 UI
  showPanel('login');
  updateUIForMode();
  
  // 添加欢迎消息
  console.log('IM 系统初始化完成');
}

// 防止意外提交表单
document.getElementById('loginForm')?.addEventListener('submit', (e) => {
  e.preventDefault();
});

init();

